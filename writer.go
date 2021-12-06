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

type middlewareWriter struct {
	http.ResponseWriter
	m        *middleware
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

func (mw *middlewareWriter) WriteHeader(status int) {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()
	mw.status = status
	mw.headerSent = true

	if cenc := mw.Header().Get("content-encoding"); cenc != "" {
		mw.ResponseWriter.WriteHeader(status)
		return
	}

	if ctype := mw.Header().Get("content-type"); ctype != "" {
		if !matchRegexes(ctype, mw.m.allowedType) {
			mw.dontEncode = true
			mw.ResponseWriter.WriteHeader(status)
			return
		}
	}

	if clen := mw.Header().Get("content-length"); clen != "" {
		len, err := strconv.ParseUint(clen, 10, 64)
		if err == nil {
			if len > mw.m.minSize {
				mw.startEncoding()
				return
			}
		}
	}

	// buffer the content until
	mw.buff = make([]byte, mw.m.minSize)
}

func (mw *middlewareWriter) startEncoding() bool {
	var err error
	mw.enc, err = mw.factory(mw.ctx, mw.ResponseWriter)
	if err != nil {
		if !mw.m.silent {
			fmt.Printf("Can not create encoder %s: %v\n", mw.encoding, err)
		}
		mw.flush()
		mw.dontEncode = true
		return false
	}
	mw.shouldEncode = true
	mw.Header().Del("content-length")
	mw.Header().Add("content-encoding", mw.encoding)
	mw.ResponseWriter.WriteHeader(mw.status)
	return true
}

func (mw *middlewareWriter) Write(chunk []byte) (int, error) {
	if !mw.headerSent {
		mw.WriteHeader(mw.status)
	}
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.shouldEncode {
		return mw.enc.Write(chunk)
	}
	if mw.dontEncode {
		return mw.ResponseWriter.Write(chunk)
	}

	newBufLen := mw.buffLen + uint64(len(chunk))
	if newBufLen > mw.m.minSize {
		if mw.startEncoding() {
			if mw.buffLen > 0 {
				_, err := mw.enc.Write(mw.buff[0:mw.buffLen])
				if err != nil {
					return 0, err
				}
			}
			mw.buff = nil
			mw.buffLen = 0
			return mw.enc.Write(chunk)
		}
		return mw.ResponseWriter.Write(chunk)
	}
	n := copy(mw.buff[mw.buffLen:], chunk[:])
	mw.buffLen = newBufLen
	return n, nil
}

func (mw *middlewareWriter) flush() {
	if mw.enc != nil {
		mw.enc.Close()
	}
	if mw.buff != nil && mw.buffLen > 0 {
		mw.ResponseWriter.Write(mw.buff[0:mw.buffLen])
	}
}
