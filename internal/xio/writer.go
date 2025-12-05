package xio

import (
	"io"
)

func NewResponseWriteCloser(w io.Writer) io.WriteCloser {
	return &responseWriteCloser{
		Writer: w,
	}
}

type responseWriteCloser struct {
	io.Writer
}

func (rwc *responseWriteCloser) Close() error {
	if closer, ok := rwc.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
