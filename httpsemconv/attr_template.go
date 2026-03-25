package httpsemconv

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// templateCtxKey is an unexported type serving the key for the URL template in the context.
type templateCtxKey struct{}

// ContextWithTemplate returns a copy of parent with template set as the current URL Template.
func ContextWithTemplate(ctx context.Context, template string) context.Context {
	return context.WithValue(ctx, templateCtxKey{}, template)
}

// TemplateFromContext returns the current URL Template from ctx.
//
// If no URL Template is currently set in ctx the empty string is returned.
func TemplateFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(templateCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// TemplateAttributeFromRequest returns a "url.template" attribute from req's context (if it has one).
// The attribute is suitable for use in either metrics or traces.
// This function is suitable for use as
func TemplateAttributeFromRequest(req *http.Request) []attribute.KeyValue {
	if template := TemplateFromContext(req.Context()); template != "" {
		return []attribute.KeyValue{semconv.URLTemplate(template)}
	}
	return nil
}

// Assert that *AttributeSetterTransport implements http.RoundTripper
var _ http.RoundTripper = (*TemplateAttributeTransport)(nil)

// NewTemplateTransport wraps the provided [http.RoundTripper] with one that sets the "url.template"
// attribute on the outgoing request context's span. This is intended to be wrapped by otelhttp.Transport
//
// If the provided base is nil, [http.DefaultTransport] will be used as the base http.RoundTripper.
func NewTemplateTransport(base http.RoundTripper) *TemplateAttributeTransport {
	return &TemplateAttributeTransport{
		Base: base,
	}
}

// TemplateAttributeTransport is a [http.RoundTripper] extracts the template from
// outgoing request's context and sets the "url.template" span attribute.
//
// This transport should be set as the base transport of otelhttp.Transport (since the
// otelhttp Transport starts and ends it's span around calling its base transport)
type TemplateAttributeTransport struct {
	Base http.RoundTripper
}

// If the context contains a URL template and a span, set it's name and the url.template attribute.
func (t *TemplateAttributeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If the context has a URL template set...
	if template := TemplateFromContext(req.Context()); template != "" {
		if span := trace.SpanFromContext(req.Context()); span.IsRecording() {
			span.SetAttributes(semconv.URLTemplate(template))
		}
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
