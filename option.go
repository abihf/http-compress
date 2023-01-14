package compress

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"regexp"
)

type Option func(*config)

func WithEncoder(encoding string, priotity int, factory EncoderFactory) Option {
	return func(m *config) {
		m.encoders[encoding] = &encoder{priority: priotity, factory: factory}
	}
}

func WihtoutEncoder(encoding string) Option {
	return func(c *config) {
		delete(c.encoders, encoding)
	}
}

func WithGzip(priority, level int) Option {
	return WithEncoder("gzip", priority, func(ctx context.Context, w io.Writer) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(w, level)
	})
}

func WithAllowedTypes(list []*regexp.Regexp) Option {
	return func(c *config) {
		c.allowedType = list
	}
}

func WithMinSize(minSize uint64) Option {
	return func(c *config) {
		c.minSize = minSize
	}
}

func WithErrorHandler(handler ErrorHandler) Option {
	return func(c *config) {
		c.errorHandler = handler
	}
}

func DefaultErrorHandler(err error, _ *http.Request, w http.ResponseWriter) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
