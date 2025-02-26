package common

import "fmt"

func NewBytesReader(b []byte) *BytesReader {
	return &BytesReader{data: b}
}

type BytesReader struct {
	data []byte
	idx  int
}

func (r *BytesReader) Read(p []byte) (n int, err error) {
	if r.idx >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.idx:])
	r.idx += n
	return n, nil
}

func (r *BytesReader) Close() error {
	return nil
}
