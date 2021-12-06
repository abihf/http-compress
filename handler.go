package compress

import (
	"net/http"
)

func Handler(h http.Handler, options ...Option) http.Handler {
	return newMiddleware(h, options...)
}
