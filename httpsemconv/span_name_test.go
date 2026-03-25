package httpsemconv_test

import (
	"net/http"
	"testing"

	"github.com/leshana/go-webutils/httpsemconv"
)

func TestSpanNameFromTemplate(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		template string
		want     string
	}{
		{
			name:     "no template returns HTTP method only",
			method:   http.MethodGet,
			template: "",
			want:     "HTTP GET",
		}, {
			name:     "GET with template",
			method:   http.MethodGet,
			template: "/users/{id}",
			want:     "HTTP GET /users/{id}",
		}, {
			name:     "PATCH with multi-param template",
			method:   http.MethodPatch,
			template: "/orgs/{orgID}/users/{id}",
			want:     "HTTP PATCH /orgs/{orgID}/users/{id}",
		}, {
			name:     "op parameter is ignored",
			method:   http.MethodGet,
			template: "/ping",
			want:     "HTTP GET /ping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := httpsemconv.ContextWithTemplate(t.Context(), tt.template)
			req, _ := http.NewRequestWithContext(ctx, tt.method, "http://example.com/ignored", nil)
			if got := httpsemconv.SpanNameFromContextTemplate("ignored-op", req); got != tt.want {
				t.Errorf("SpanNameFromTemplate():\nwant: %q\ngot: %q", tt.want, got)
			}
		})
	}
}

// Validate that crntel.SpanNameFromPattern returns the correct string for all types of patterns.
func TestSpanNameFromPattern(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		pattern string
		want    string
	}{
		// Empty pattern: fall back to op.
		{
			name:    "empty pattern returns op",
			method:  http.MethodGet,
			pattern: "",
			want:    "op",
		},
		// PATH only patterns.
		{
			name:    "root path",
			method:  http.MethodGet,
			pattern: "/",
			want:    "GET /",
		}, {
			name:    "simple path",
			method:  http.MethodGet,
			pattern: "/foo/bar",
			want:    "GET /foo/bar",
		}, {
			name:    "path with trailing slash",
			method:  http.MethodPost,
			pattern: "/foo/bar/",
			want:    "POST /foo/bar/",
		}, {
			name:    "path with named wildcard",
			method:  http.MethodGet,
			pattern: "/foo/{id}",
			want:    "GET /foo/{id}",
		}, {
			name:    "path with catch-all wildcard",
			method:  http.MethodGet,
			pattern: "/foo/{name...}",
			want:    "GET /foo/{name...}",
		}, {
			name:    "path with dollar wildcard",
			method:  http.MethodGet,
			pattern: "/foo/{$}",
			want:    "GET /foo/{$}",
		},
		// HOST/PATH patterns (no METHOD).
		{
			name:    "host and path",
			method:  http.MethodGet,
			pattern: "example.com/foo/bar",
			want:    "GET example.com/foo/bar",
		}, {
			name:    "host and path with wildcard",
			method:  http.MethodPut,
			pattern: "example.com/foo/{id}",
			want:    "PUT example.com/foo/{id}",
		},
		// METHOD + PATH patterns: pattern METHOD is ignored, request method is used.
		{
			name:    "pattern method matches request method",
			method:  http.MethodGet,
			pattern: "GET /foo/bar",
			want:    "GET /foo/bar",
		}, {
			name:    "pattern method differs from request method",
			method:  http.MethodGet,
			pattern: "POST /foo/bar",
			want:    "GET /foo/bar",
		}, {
			name:    "pattern method with wildcard path",
			method:  http.MethodDelete,
			pattern: "DELETE /foo/{id}",
			want:    "DELETE /foo/{id}",
		},
		// METHOD + HOST + PATH patterns.
		{
			name:    "pattern method with host and path",
			method:  http.MethodGet,
			pattern: "GET example.com/foo/bar",
			want:    "GET example.com/foo/bar",
		}, {
			name:    "pattern method with host and wildcard path",
			method:  http.MethodPatch,
			pattern: "POST example.com/foo/{id}",
			want:    "PATCH example.com/foo/{id}",
		},
		// Tab separator between METHOD and HOST/PATH.
		{
			name:    "tab-separated method and path",
			method:  http.MethodPost,
			pattern: "GET\t/foo/bar",
			want:    "POST /foo/bar",
		},
		// Invalid pattern: don't panic
		{
			name:    "invalid syntax no space",
			method:  http.MethodPost,
			pattern: "GET/things\t",
			want:    "POST GET/things\t",
		}, {
			name:    "invalid syntax no slash",
			method:  http.MethodPost,
			pattern: "things",
			want:    "POST things",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{Method: tc.method, Pattern: tc.pattern}
			got := httpsemconv.SpanNameFromPattern("op", r)
			if got != tc.want {
				t.Errorf("SpanNameFromPattern(%q, {Method:%q, Pattern:%q}):\ngot  %q\nwant %q", "op",
					tc.method, tc.pattern, got, tc.want)
			}
		})
	}
}
