package httpsemconv_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/leshana/go-webutils/httpsemconv"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// TestContextWithTemplate_RoundTrip verifies that ContextWithTemplate stores the template
// and TemplateFromContext retrieves it.
func TestContextWithTemplate_TemplateFromContext(t *testing.T) {
	ctx := t.Context()
	if got := httpsemconv.TemplateFromContext(ctx); got != "" {
		t.Errorf("TemplateFromContext(empty ctx) = %q, want %q", got, "")
	}

	const template = "/users/{id}"
	ctx = httpsemconv.ContextWithTemplate(ctx, template)
	if got := httpsemconv.TemplateFromContext(ctx); got != template {
		t.Errorf("TemplateFromContext after ContextWithTemplate = %q, want %q", got, template)
	}
}

func TestTemplateAttributeFromRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.org/path?param1=val1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should return no attributes by default
	if got := httpsemconv.TemplateAttributeFromRequest(req); len(got) != 0 {
		t.Errorf("TemplateAttributeFromRequest() = %v want []", got)
	}

	// But if we put a template url into the context it should return "url.template"
	const template = "/users/{id}"
	req = req.Clone(httpsemconv.ContextWithTemplate(t.Context(), template))
	got := httpsemconv.TemplateAttributeFromRequest(req)

	want := semconv.URLTemplate(template)
	if len(got) != 1 {
		t.Errorf("TemplateAttributeFromRequest() = %v want %v", got, want)
	} else if actual := got[0]; actual != want {
		t.Errorf("TemplateAttributeFromRequest() = %v want %v", got, want)
	}
}

// roundTripFunc is a test helper that adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// mockOKTransport returns a RoundTripper that always responds 200 OK.
var mockOKTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Request: r, Body: http.NoBody}, nil
})

func TestTemplateAttributeTransport_RoundTrip(t *testing.T) {
	const method = http.MethodGet
	const template = "/users/{id}"

	// Create a mock tracer provider that records all the spans made on it.
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tracer := tp.Tracer("test")

	// Instantiate our template transport wrapped around a mock that always returns 200 OK
	inst := httpsemconv.NewTemplateTransport(mockOKTransport)

	// Instead of using the real otelhttp.Transport we can fake one up and save the dependency
	pretendOTelHttpTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// It would create a span, invoke the base, then return
		ctx, span := tracer.Start(r.Context(), "initial-name")
		r = r.Clone(ctx)
		res, err := inst.RoundTrip(r)
		span.End()
		return res, err
	})
	// Then an httpClient which uses it.
	httpClient := http.Client{Transport: pretendOTelHttpTransport}

	tests := []struct {
		name       string
		makeReqCtx func(t *testing.T) context.Context
		wantName   string
		wantAttr   string
	}{
		{
			name:     "with URL template in context",
			wantName: "HTTP GET /users/{id}",
			wantAttr: template,
			makeReqCtx: func(t *testing.T) context.Context {
				return httpsemconv.ContextWithTemplate(t.Context(), template)
			},
		},
		{
			name:     "without URL template in context",
			wantName: "initial-name",
			wantAttr: "",
			makeReqCtx: func(t *testing.T) context.Context {
				return t.Context()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.Reset()

			ctx := tt.makeReqCtx(t)
			req, _ := http.NewRequestWithContext(ctx, method, "http://example.com/users/123", nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Fatalf("RoundTrip error: %v", err)
			} else if resp.StatusCode != http.StatusOK {
				t.Errorf("unexpected status: %d", resp.StatusCode)
			}

			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("expected 1 recorded span, got %d", len(spans))
			}
			recorded := spans[0]

			// if got := recorded.Name(); got != tt.wantName {
			// 	t.Errorf("span.Name() = %q, want %q", got, tt.wantName)
			// }

			var templateAttr string
			for _, a := range recorded.Attributes() {
				if a.Key == semconv.URLTemplateKey {
					templateAttr = a.Value.AsString()
				}
			}
			if templateAttr != tt.wantAttr {
				t.Errorf("url.template attribute = %q, want %q", templateAttr, tt.wantAttr)
			}
		})
	}

	t.Run("template in context with non-recording span does not panic", func(t *testing.T) {
		recorder.Reset()

		// For this test we directly invoke the round-tripper without a wrapping otelhttp.Transport
		// That way the ctx has no span (noop), so IsRecording() is false.
		ctx := t.Context()
		req, _ := http.NewRequestWithContext(ctx, method, "http://example.com/users/123", nil)
		resp, err := inst.RoundTrip(req)

		if err != nil {
			t.Fatalf("RoundTrip error: %v", err)
		} else if resp.StatusCode != http.StatusOK {
			t.Errorf("unexpected status: %d", resp.StatusCode)
		}
	})
}
