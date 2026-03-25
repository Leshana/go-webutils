package otelhttp_transport_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leshana/go-webutils/httpsemconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func TestTemplateAttributeTransport(t *testing.T) {
	// Dummy test server to serve as a test target
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello %s!\n", strings.TrimPrefix(r.URL.Path, "/hello"))
	}))
	t.Cleanup(testServer.Close)

	// Create a mock tracer provider that records all the spans made on it.
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer tp.Shutdown(context.Background())

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	// httpClient instrumented by otelhttp and our template transport
	httpClient := http.Client{
		Transport: otelhttp.NewTransport(
			httpsemconv.NewTemplateTransport(nil),
			otelhttp.WithTracerProvider(tp),
			otelhttp.WithMeterProvider(mp),
			otelhttp.WithSpanNameFormatter(httpsemconv.SpanNameFromContextTemplate),
			otelhttp.WithMetricAttributesFn(httpsemconv.TemplateAttributeFromRequest),
		),
	}

	// Example request-making function:
	callHello := func(ctx context.Context, name string) error {
		const template = "/hello/{name}"
		ctx = httpsemconv.ContextWithTemplate(ctx, template)
		reqURL := testServer.URL + strings.ReplaceAll(template, "{name}", name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	// invoke our example function
	err := callHello(t.Context(), "bob")
	if err != nil {
		t.Errorf("callHello: expected nil error but returned %v", err)
	}

	// Verify all the telemetry we expect actually got recorded.
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 recorded span, got %d", len(spans))
	}
	recorded := spans[0]

	// Validate recorded span's name
	const wantName = "HTTP GET /hello/{name}"
	if got := recorded.Name(); got != wantName {
		t.Errorf("span.Name() = %q, want %q", got, wantName)
	}

	// Validate the url.template attribute on the span
	const wantTemplateAttr = "/hello/{name}"
	if templateAttr, _ := getAttribute(recorded, semconv.URLTemplateKey); templateAttr != wantTemplateAttr {
		t.Errorf("url.template attribute = %q, want %q", templateAttr, wantTemplateAttr)
	}

	// 3. Collect snapshot of the current metric state
	var actualData metricdata.ResourceMetrics
	if err = reader.Collect(t.Context(), &actualData); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	// Validate that the metrics have the url.template attribute too
	metricdatatest.AssertHasAttributes(t, actualData, semconv.URLTemplate("/hello/{name}"))
}

// getAttribute gets the value of the attribute with key, or the empty string.
func getAttribute(recorded sdktrace.ReadOnlySpan, key attribute.Key) (string, bool) {
	for _, a := range recorded.Attributes() {
		if a.Key == key {
			return a.Value.AsString(), true
		}
	}
	return "", false
}
