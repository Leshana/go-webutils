// Package httpsemconv provides helpers for following with OpenTelemetry (OTEL) semantic
// conventions for HTTP traces, metrics, and logs.
package httpsemconv

import (
	"net/http"
	"strings"
)

// SpanNameFromContextTemplate formats an OTEL Span name for HTTP client requests
// using the URL Template carried by the request context.
func SpanNameFromContextTemplate(_ string, req *http.Request) string {
	if template := TemplateFromContext(req.Context()); template != "" {
		return "HTTP " + req.Method + " " + template
	}
	return "HTTP " + req.Method
}

// SpanNameFromPattern formats an OTEL Span name for HTTP server requests using [http.Request.Pattern] as {target}
//
// If r.Pattern is present it is parsed assuming it has the same syntax accepted by [http.ServeMux]:
//   - The returned name is generally of the form "{method} {target}" where target is the pattern's PATH.
//   - If the pattern has a METHOD it is ignored: The actual method of the request is used.
//   - If the pattern has a HOST it is treated as being part of the {target}
func SpanNameFromPattern(op string, r *http.Request) string {
	if r.Pattern == "" {
		return op
	}
	// If there is a space (or tab) prior to the /, then the text before that must be METHOD
	slashPos := strings.IndexByte(r.Pattern, '/')
	if slashPos > 0 {
		if spacePos := strings.IndexAny(r.Pattern[:slashPos], " \t"); spacePos >= 0 {
			return r.Method + " " + r.Pattern[spacePos+1:] // There was a method part: take only the portion after it
		}
	}
	return r.Method + " " + r.Pattern // There was no method part so use the whole pattern.
}
