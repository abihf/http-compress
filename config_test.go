package compress

import (
	"context"
	"io"
	"testing"

	"gotest.tools/assert"
)

func Test_populateSupportedEncoding(t *testing.T) {
	var dummyFactory EncoderFactory = func(ctx context.Context, w io.Writer) (io.WriteCloser, error) { return nil, nil }
	m := newConfig(WithEncoder("a", 1, dummyFactory), WithEncoder("b", 2, dummyFactory), WithEncoder("c", 3, dummyFactory), WihtoutEncoder("gzip"))
	assert.DeepEqual(t, m.supportedEncoding, []string{"a", "b", "c"})
}
