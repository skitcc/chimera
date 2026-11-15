package middleware

import (
	"net/http"

	"chimera/platform/grpcx"
)

const (
	HeaderInstance  = "X-Chimera-Instance"
	HeaderUpstreams = "X-Chimera-Upstreams"
)

// Instance names the Gateway pod in X-Chimera-Instance and the service pods that
// answered the request's gRPC calls in X-Chimera-Upstreams.
func Instance(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if name != "" {
				w.Header().Set(HeaderInstance, name)
			}
			ctx, upstreams := grpcx.WithUpstreams(r.Context())
			next.ServeHTTP(&upstreamWriter{ResponseWriter: w, upstreams: upstreams}, r.WithContext(ctx))
		})
	}
}

// upstreamWriter adds X-Chimera-Upstreams right before the status line goes out,
// when every call the handler made has answered.
type upstreamWriter struct {
	http.ResponseWriter
	upstreams *grpcx.Upstreams
	wrote     bool
}

func (w *upstreamWriter) WriteHeader(status int) {
	if !w.wrote {
		w.wrote = true
		if names := w.upstreams.String(); names != "" {
			w.Header().Set(HeaderUpstreams, names)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *upstreamWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *upstreamWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
