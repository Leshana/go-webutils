# otelhttp_transport
This example demonstrates how the httpsemconv package's helpers can add "url.template" 
attributes to HTTP client requests instrumented by the 
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp module.

```go
httpClient := http.Client{
    Transport: otelhttp.NewTransport(
        httpsemconv.NewTemplateTransport(nil),
        otelhttp.WithSpanNameFormatter(httpsemconv.SpanNameFromContextTemplate),
        otelhttp.WithMetricAttributesFn(httpsemconv.TemplateAttributeFromRequest),
    ),
}
```

```go
const template = "/hello/{name}"
ctx = httpsemconv.ContextWithTemplate(ctx, template)
req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.org/hello/joe", nil)
// ...
```