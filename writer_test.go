package compress

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"gotest.tools/assert"
)

func Test_withKnowContentEncoding(t *testing.T) {
	w := newMockRW()
	c := newConfig()
	encoding := "gzip"
	rw := responseWriter{
		ResponseWriter: w,
		c:              c,
		ctx:            context.Background(),
		factory:        c.encoders[encoding].factory,
		encoding:       encoding,
	}

	rw.Header().Set("content-encoding", "flate")
	rw.Header().Set("content-type", "text/plain")
	rw.WriteHeader(200)

	assert.Equal(t, rw.dontEncode, true)
	assert.Equal(t, rw.shouldEncode, false)
}

func Test_withKnowContentLength(t *testing.T) {
	w := newMockRW()
	c := newConfig()
	encoding := "gzip"
	rw := responseWriter{
		ResponseWriter: w,
		c:              c,
		ctx:            context.Background(),
		factory:        c.encoders[encoding].factory,
		encoding:       encoding,
	}

	rw.Header().Set("content-type", "text/plain")
	rw.Header().Set("content-length", "100000")
	rw.WriteHeader(400)

	assert.Equal(t, rw.dontEncode, false)
	assert.Equal(t, rw.shouldEncode, true)
	assert.Equal(t, rw.status, 400)
}

type mockResponseWriter struct {
	h      http.Header
	status int
	body   bytes.Buffer
}

func newMockRW() http.ResponseWriter {
	return &mockResponseWriter{
		status: 200,
		body:   bytes.Buffer{},
		h:      http.Header{},
	}
}

func (w *mockResponseWriter) Header() http.Header {
	return w.h
}

func (w *mockResponseWriter) Write(c []byte) (int, error) {
	return w.body.Write(c)
}

func (w *mockResponseWriter) WriteHeader(status int) {
	w.status = status
}
