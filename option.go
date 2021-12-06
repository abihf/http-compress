package compress

import (
	"compress/gzip"
	"context"
	"io"
	"regexp"
)

type Option func(m *middleware)

func WithEncoder(encoding string, priotity int, factory EncoderFactory) Option {
	return func(m *middleware) {
		m.encoders[encoding] = &encoder{priority: priotity, factory: factory}
	}
}

func WihtoutEncoder(encoding string) Option {
	return func(m *middleware) {
		delete(m.encoders, encoding)
	}
}

func WithGzip(priority, level int) Option {
	return WithEncoder("gzip", priority, func(ctx context.Context, w io.Writer) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(w, level)
	})
}

func WithAllowedTypes(list []*regexp.Regexp) Option {
	return func(m *middleware) {
		m.allowedType = list
	}
}

func WithMinSize(minSize uint64) Option {
	return func(m *middleware) {
		m.minSize = minSize
	}
}

func WithSilent() Option {
	return func(m *middleware) {
		m.silent = true
	}
}
