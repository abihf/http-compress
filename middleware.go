package compress

import (
	"net/http"

	"github.com/kevinpollet/nego"
)

type Middleware func(http.Handler) http.Handler

func New(options ...Option) Middleware {
	conf := newConfig(options...)
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Accept-Encoding")

			encoding := nego.NegotiateContentEncoding(r, conf.supportedEncoding...)
			if mw, ok := newResponseWriter(r, w, conf, encoding); ok {
				defer mw.end()
				w = mw
			}
			h.ServeHTTP(w, r)
		})
	}
}

func Handler(h http.Handler, options ...Option) http.Handler {
	return New(options...)(h)
}
