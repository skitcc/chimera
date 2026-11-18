package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HeaderRequestID returns the trace id of the request: the correlation id to look
// up in logs and traces.
const HeaderRequestID = "X-Request-Id"

// Trace runs inside the router under the server span otelhttp opened. It answers
// with the trace id and names the span after the matched route once it is known.
func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if sc := span.SpanContext(); sc.IsValid() {
			w.Header().Set(HeaderRequestID, sc.TraceID().String())
		}
		next.ServeHTTP(w, r)
		route := routePattern(r)
		span.SetName(r.Method + " " + route)
		span.SetAttributes(attribute.String("http.route", route))
	})
}
