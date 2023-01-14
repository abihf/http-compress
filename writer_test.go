package compress

import (
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

func Test_withKnowContentEncoding(t *testing.T) {
	w := httptest.NewRecorder()
	c := newConfig()
	rw, ok := newResponseWriter(nil, w, c, "gzip")
	assert.Equal(t, ok, true)

	rw.Header().Set("content-encoding", "flate")
	rw.Header().Set("content-type", "text/plain")
	rw.WriteHeader(401)
	rw.end()

	assert.Equal(t, w.Code, 401)
	assert.Equal(t, w.Header().Get("content-encoding"), "flate")
}

func Test_withKnowContentLength(t *testing.T) {
	w := httptest.NewRecorder()
	c := newConfig()
	rw, ok := newResponseWriter(nil, w, c, "gzip")
	assert.Equal(t, ok, true)

	rw.Header().Set("content-type", "text/plain")
	rw.Header().Set("content-length", "100000")
	rw.WriteHeader(400)
	rw.end()

	assert.Equal(t, w.Code, 400)
	assert.Equal(t, w.Header().Get("content-encoding"), "gzip")
	assert.Equal(t, w.Header().Get("content-length"), "")
}

func Test_withUnknownContentLength(t *testing.T) {
	run := func(bodyWriteTimes int, status int, encoding string, bodyLen int) func(*testing.T) {
		return func(t *testing.T) {
			w := httptest.NewRecorder()
			c := newConfig(WithMinSize(15))
			rw, ok := newResponseWriter(nil, w, c, "gzip")
			assert.Equal(t, ok, true)

			rw.Header().Set("content-type", "text/plain")
			rw.WriteHeader(status)
			body := make([]byte, 10)
			for i := 0; i < bodyWriteTimes; i++ {
				rw.Write(body)
			}
			rw.end()

			assert.Equal(t, w.Code, status)
			assert.Equal(t, w.Header().Get("content-encoding"), encoding)
			assert.Equal(t, w.Body.Len(), bodyLen)
		}
	}

	t.Run("less than minSize", run(1, 403, "", 10))
	t.Run("more than minSize", run(2, 403, "gzip", 27))
}
