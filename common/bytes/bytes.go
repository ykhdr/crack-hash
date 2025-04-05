package bytes

import (
	"io"
)

func NewReader(b []byte) *Reader {
	return &Reader{data: b}
}

type Reader struct {
	data []byte
	idx  int
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if r.idx >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.idx:])
	r.idx += n
	return n, nil
}

func (r *Reader) Close() error {
	return nil
}
