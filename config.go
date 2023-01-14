package compress

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"regexp"
	"sort"

	"github.com/jackc/puddle/v2"
)

var DefaultAllowedTypes = []*regexp.Regexp{
	regexp.MustCompile(`^text/`),
	regexp.MustCompile(`^application/json`),
	regexp.MustCompile(`^application/javascript`),
	regexp.MustCompile(`\+(xml|json)$`),
	regexp.MustCompile(`^image/svg`),
}

type config struct {
	supportedEncoding []string

	encoders    map[string]*encoder
	allowedType []*regexp.Regexp
	minSize     uint64
	errorHandler ErrorHandler

	buffPull *puddle.Pool[[]byte]
}

type EncoderFactory func(ctx context.Context, w io.Writer) (io.WriteCloser, error)
type ErrorHandler func(error, *http.Request, http.ResponseWriter)

type encoder struct {
	priority int
	factory  EncoderFactory
}

func newConfig(options ...Option) *config {
	c := &config{
		minSize: 4 * 1024, 
		allowedType: DefaultAllowedTypes, 
		encoders: map[string]*encoder{},
		errorHandler: DefaultErrorHandler,
	}
	WithGzip(100, gzip.DefaultCompression)(c)
	for _, o := range options {
		o(c)
	}
	c.populateSupportedEncoding()

	pool, _ := puddle.NewPool(&puddle.Config[[]byte]{
		MaxSize: 100000, // let's hardcoded it for now
		Constructor: func(ctx context.Context) (res []byte, err error) {
			return make([]byte, c.minSize), nil
		},
	})
	c.buffPull = pool
	return c
}

func (c *config) populateSupportedEncoding() {
	type encoderWithName struct {
		*encoder
		name string
	}

	encList := make([]encoderWithName, 0, len(c.encoders))
	for name, enc := range c.encoders {
		encList = append(encList, encoderWithName{enc, name})
	}
	sort.Slice(encList, func(i, j int) bool {
		return encList[i].priority < encList[j].priority
	})
	c.supportedEncoding = make([]string, 0, len(encList))
	for _, enc := range encList {
		c.supportedEncoding = append(c.supportedEncoding, enc.name)
	}
}
