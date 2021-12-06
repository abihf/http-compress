package compress

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
)

type responseWriter struct {
	http.ResponseWriter
	c        *config
	ctx      context.Context
	factory  EncoderFactory
	enc      io.WriteCloser
	encoding string

	dontEncode   bool
	shouldEncode bool

	headerSent bool
	status     int

	buff    []byte
	buffLen uint64

	mutex sync.Mutex
}

func matchRegexes(str string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(str) {
			return true
		}
	}
	return false
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.mutex.Lock()
	defer rw.mutex.Unlock()
	rw.status = status
	rw.headerSent = true

	cenc := rw.Header().Get("content-encoding")
	ctype := rw.Header().Get("content-type")

	// if content encoding already defined
	// or content type is not defined
	// or content type is not in allowed list
	// => just forward the body
	if cenc != "" || ctype == "" || !matchRegexes(ctype, rw.c.allowedType) {
		rw.dontEncode = true
		rw.ResponseWriter.WriteHeader(status)
		return
	}

	// if content length is big enough, start encoding now
	if clen := rw.Header().Get("content-length"); clen != "" {
		len, err := strconv.ParseUint(clen, 10, 64)
		if err == nil {
			if len >= rw.c.minSize {
				rw.startEncoding()
			} else {
				rw.dontEncode = true
				rw.ResponseWriter.WriteHeader(status)
			}
			return
		}
	}

	// the content length is unknown, buffer the response until it exceeds minSize
	rw.buff = make([]byte, rw.c.minSize)
}

func (rw *responseWriter) startEncoding() bool {
	var err error
	rw.enc, err = rw.factory(rw.ctx, rw.ResponseWriter)
	if err != nil {
		if !rw.c.silent {
			fmt.Printf("Can not create encoder %s: %v\n", rw.encoding, err)
		}
		rw.flush()
		rw.dontEncode = true
		return false
	}
	rw.shouldEncode = true
	rw.Header().Del("content-length")
	rw.Header().Add("content-encoding", rw.encoding)
	rw.ResponseWriter.WriteHeader(rw.status)
	return true
}

func (rw *responseWriter) Write(chunk []byte) (int, error) {
	if !rw.headerSent {
		rw.WriteHeader(rw.status)
	}
	rw.mutex.Lock()
	defer rw.mutex.Unlock()

	if rw.shouldEncode {
		return rw.enc.Write(chunk)
	}
	if rw.dontEncode {
		return rw.ResponseWriter.Write(chunk)
	}

	newBufLen := rw.buffLen + uint64(len(chunk))
	if newBufLen > rw.c.minSize {
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

func (rw *responseWriter) flush() {
	if rw.enc != nil {
		rw.enc.Close()
	}
	if rw.buff != nil && rw.buffLen > 0 {
		rw.ResponseWriter.Write(rw.buff[0:rw.buffLen])
	}
}
