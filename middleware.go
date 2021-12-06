package compress

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"regexp"
	"sort"

	"github.com/kevinpollet/nego"
)

type EncoderFactory func(ctx context.Context, w io.Writer) (io.WriteCloser, error)

type encoder struct {
	priority int
	factory  EncoderFactory
}

type middleware struct {
	http.Handler

	supportedEncoding []string

	encoders    map[string]*encoder
	allowedType []*regexp.Regexp
	minSize     uint64
	silent      bool
}

var DefaultAllowedTypes = []*regexp.Regexp{
	regexp.MustCompile(`^text/`),
	regexp.MustCompile(`^application/json`),
	regexp.MustCompile(`^application/javascript`),
	regexp.MustCompile(`\+(xml|json)$`),
	regexp.MustCompile(`^image/svg`),
}

func newMiddleware(h http.Handler, options ...Option) *middleware {
	m := &middleware{Handler: h, minSize: 4 * 1024, allowedType: DefaultAllowedTypes, encoders: map[string]*encoder{}}
	WithGzip(100, gzip.DefaultCompression)(m)
	for _, o := range options {
		o(m)
	}
	m.populateSupportedEncoding()
	return m
}

func (m *middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	encoding := nego.NegotiateContentEncoding(r, m.supportedEncoding...)
	enc, ok := m.encoders[encoding]
	if !ok {
		m.Handler.ServeHTTP(w, r)
		return
	}

	mw := &middlewareWriter{
		ResponseWriter: w,
		ctx:            r.Context(),
		factory:        enc.factory,
		encoding:       encoding,
		m:              m,
		status:         http.StatusOK,
	}
	defer mw.flush()

	m.Handler.ServeHTTP(mw, r)

}

func (m *middleware) populateSupportedEncoding() {
	type encoderWithName struct {
		*encoder
		name string
	}

	encList := make([]encoderWithName, 0, len(m.encoders))
	for name, enc := range m.encoders {
		encList = append(encList, encoderWithName{enc, name})
	}
	sort.Slice(encList, func(i, j int) bool {
		return encList[i].priority < encList[j].priority
	})
	m.supportedEncoding = make([]string, 0, len(encList))
	for _, enc := range encList {
		m.supportedEncoding = append(m.supportedEncoding, enc.name)
	}
}
