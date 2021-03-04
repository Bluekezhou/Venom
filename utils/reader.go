package utils

import (
	"errors"
	"fmt"
	"io"
)

// CancelableReader 定义
type CancelableReader struct {
	r      io.Reader
	data   chan []byte
	done   chan bool
	cancel chan bool
	exit   bool
	size   int
}

// NewCancelableReader 创建一个Reader
func NewCancelableReader(input io.Reader, bufsize int) *CancelableReader {
	return &CancelableReader{
		r:      input,
		data:   make(chan []byte, bufsize),
		done:   make(chan bool),
		cancel: make(chan bool),
		exit:   false,
		size:   bufsize,
	}
}

// StartReader 启动独立read线程
func (cr *CancelableReader) StartReader() {
	go func() {
		buf := make([]byte, cr.size)
		for !cr.exit {
			count, err := cr.r.Read(buf)
			if err != nil {
				return
			}

			cr.data <- buf[:count]
		}
	}()
}

// Read data from wrapped io
func (cr *CancelableReader) Read(p []byte) (n int, err error) {
	select {
	case out := <-cr.data:
		copy(p, out)
		return len(out), nil
	case <-cr.done:
		cr.exit = true
		return 0, errors.New("input reader is closed")
	case <-cr.cancel:
		return 0, errors.New("current read is cancelled")
	}
}

// SendCancelMessage 给输入线程发送Cancel消息
func (cr *CancelableReader) SendCancelMessage() {
	if !cr.exit {
		cr.cancel <- true
	} else {
		fmt.Println("SendCancelMessage was called twice, check it")
	}
}

// GetBufSize 获取缓冲区大小
func (cr *CancelableReader) GetBufSize() int {
	return cr.size
}

// StdReader 全局的CancelableReader
var StdReader *CancelableReader = nil
