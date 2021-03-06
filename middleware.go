package compress

import (
	"context"
	"io"
	"net/http"

	"github.com/kevinpollet/nego"
)

type EncoderFactory func(ctx context.Context, w io.Writer) (io.WriteCloser, error)

type encoder struct {
	priority int
	factory  EncoderFactory
}

type Middleware func(http.Handler) http.Handler

func New(options ...Option) Middleware {
	c := newConfig(options...)
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Accept-Encoding")

			encoding := nego.NegotiateContentEncoding(r, c.supportedEncoding...)
			enc, ok := c.encoders[encoding]
			if ok {
				mw := &responseWriter{
					ResponseWriter: w,
					ctx:            r.Context(),
					factory:        enc.factory,
					encoding:       encoding,
					c:              c,
					status:         http.StatusOK,
				}
				defer mw.flush()
				w = mw
			}
			h.ServeHTTP(w, r)
		})
	}
}

func Handler(h http.Handler, options ...Option) http.Handler {
	return New(options...)(h)
}
