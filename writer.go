package compress

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/jackc/puddle/v2"
)

var (
	_ http.ResponseWriter = &responseWriter{}
	_ http.Flusher        = &responseWriter{}
)

type responseWriter struct {
	http.ResponseWriter

	err error
	req *http.Request

	conf     *config
	ctx      context.Context
	factory  EncoderFactory
	enc      io.WriteCloser
	encoding string

	dontEncode   bool
	shouldEncode bool

	headerSent bool
	statusSent bool
	status     int

	buff    *puddle.Resource[[]byte]
	buffLen uint64
}

func newResponseWriter(r *http.Request, w http.ResponseWriter, c *config, encoding string) (*responseWriter, bool) {
	enc, ok := c.encoders[encoding]
	if !ok {
		return nil, false
	}
	var ctx context.Context
	if r == nil {
		ctx = context.Background()
	} else {
		ctx = r.Context()
	}
	return &responseWriter{
		ResponseWriter: w,

		req:      r,
		ctx:      ctx,
		factory:  enc.factory,
		encoding: encoding,
		conf:     c,
		status:   http.StatusOK,
	}, true
}

func matchRegexes(str string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(str) {
			return true
		}
	}
	return false
}

func (rw *responseWriter) sendStatus() {
	if !rw.statusSent && rw.err == nil {
		rw.statusSent = true
		rw.ResponseWriter.WriteHeader(rw.status)
	}
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.writeHeader()
}

func (rw *responseWriter) writeHeader() {
	if rw.headerSent {
		return
	}
	rw.headerSent = true

	cenc := rw.Header().Get("content-encoding")
	ctype := rw.Header().Get("content-type")

	// dont encode if:
	// * content encoding already defined
	// * content type is not in allowed list
	if cenc != "" || ctype == "" || !matchRegexes(ctype, rw.conf.allowedType) {
		rw.dontEncode = true
		rw.sendStatus()
		return
	}

	// if content length is big enough, start encoding now
	if clen := rw.Header().Get("content-length"); clen != "" {
		if len, err := strconv.ParseUint(clen, 10, 64); err == nil {
			if len >= rw.conf.minSize {
				rw.startEncoding()
			} else {
				rw.dontEncode = true
				rw.sendStatus()
			}
			return
		}
	}

	// the content length is unknown, buffer the response until it exceeds minSize
	buff, err := rw.conf.buffPull.Acquire(context.Background())
	if err != nil {
		rw.err = err
		return
	}
	rw.buff = buff
}

func (rw *responseWriter) startEncoding() bool {
	var err error
	rw.enc, err = rw.factory(rw.ctx, rw.ResponseWriter)
	if err != nil {
		rw.err = err
		rw.end()
		rw.dontEncode = true
		return false
	}
	rw.shouldEncode = true
	rw.Header().Del("content-length")
	rw.Header().Set("content-encoding", rw.encoding)
	rw.sendStatus()
	return true
}

func (rw *responseWriter) Write(chunk []byte) (int, error) {
	if rw.err != nil {
		return 0, rw.err
	}

	rw.writeHeader()

	if rw.shouldEncode {
		return rw.enc.Write(chunk)
	}
	if rw.dontEncode {
		return rw.ResponseWriter.Write(chunk)
	}

	newBufLen := rw.buffLen + uint64(len(chunk))
	if newBufLen > rw.conf.minSize {
		if rw.startEncoding() {
			if rw.buffLen > 0 {
				_, err := rw.enc.Write(rw.buff.Value()[0:rw.buffLen])
				if err != nil {
					return 0, err
				}
			}
			rw.buff.Release()
			rw.buff = nil
			rw.buffLen = 0
			return rw.enc.Write(chunk)
		}
		return rw.ResponseWriter.Write(chunk)
	}
	n := copy(rw.buff.Value()[rw.buffLen:], chunk)
	rw.buffLen = newBufLen
	return n, nil
}

func (rw *responseWriter) end() {
	if rw.buff != nil {
		defer rw.buff.Release()
	}

	hasError := rw.err != nil
	if hasError {
		rw.conf.errorHandler(rw.err, rw.req, rw.ResponseWriter)
	} else {
		rw.sendStatus()
	}

	if rw.enc != nil {
		rw.enc.Close()
	}

	if !hasError && rw.buff != nil && rw.buffLen > 0 {
		buff := rw.buff.Value()
		rw.ResponseWriter.Write(buff[0:rw.buffLen])
	}
}

func (rw *responseWriter) Flush() {
	if rw.err != nil {
		return
	}

	if rw.shouldEncode {
		flushWriter(rw.enc)
		flushWriter(rw.ResponseWriter)
		return
	}

	if rw.buffLen > 0 {
		rw.ResponseWriter.Write(rw.buff.Value()[0:rw.buffLen])
		rw.dontEncode = true
		rw.buff.Release()
		rw.buff = nil
		rw.buffLen = 0
	}
	flushWriter(rw.ResponseWriter)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func flushWriter(w interface{}) {
	// gzip Flush() returns error
	type flusherWithErr interface {
		Flush() error
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else if f, ok := w.(flusherWithErr); ok {
		f.Flush()
	}
}
