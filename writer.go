package compress

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

var (
	_ http.ResponseWriter = &responseWriter{}
	_ http.Flusher        = &responseWriter{}
)

type responseWriter struct {
	http.ResponseWriter

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

	buff    []byte
	buffLen uint64
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
	if !rw.statusSent {
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

	// if content encoding already defined
	// or content type is not defined
	// or content type is not in allowed list
	// => just forward the body
	if cenc != "" || ctype == "" || !matchRegexes(ctype, rw.conf.allowedType) {
		rw.dontEncode = true
		rw.sendStatus()
		return
	}

	// if content length is big enough, start encoding now
	if clen := rw.Header().Get("content-length"); clen != "" {
		len, err := strconv.ParseUint(clen, 10, 64)
		if err == nil {
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
	rw.buff = make([]byte, rw.conf.minSize)
}

func (rw *responseWriter) startEncoding() bool {
	var err error
	rw.enc, err = rw.factory(rw.ctx, rw.ResponseWriter)
	if err != nil {
		if !rw.conf.silent {
			fmt.Printf("Can not create encoder %s: %v\n", rw.encoding, err)
		}
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
				_, err := rw.enc.Write(rw.buff[0:rw.buffLen])
				if err != nil {
					return 0, err
				}
			}
			rw.buff = nil
			rw.buffLen = 0
			return rw.enc.Write(chunk)
		}
		return rw.ResponseWriter.Write(chunk)
	}
	n := copy(rw.buff[rw.buffLen:], chunk)
	rw.buffLen = newBufLen
	return n, nil
}

func (rw *responseWriter) end() {
	rw.sendStatus()

	if rw.enc != nil {
		rw.enc.Close()
	}
	if rw.buff != nil && rw.buffLen > 0 {
		rw.ResponseWriter.Write(rw.buff[0:rw.buffLen])
	}
}

func (rw *responseWriter) Flush() {
	if rw.shouldEncode {
		flushWriter(rw.enc)
		flushWriter(rw.ResponseWriter)
		return
	}

	if rw.buffLen > 0 {
		rw.ResponseWriter.Write(rw.buff[0:rw.buffLen])
		rw.dontEncode = true
		rw.buff = nil
		rw.buffLen = 0
	}
	flushWriter(rw.ResponseWriter)
}

// gzip Flush() returns error
type flusherWithErr interface {
	Flush() error
}

func flushWriter(w interface{}) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else if f, ok := w.(flusherWithErr); ok {
		f.Flush()
	}
}
