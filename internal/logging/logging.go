package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Writer struct {
	queue  chan []byte
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

func Open(base string) (*Writer, error) {
	dir := filepath.Join(base, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "launcher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	w := &Writer{queue: make(chan []byte, 256), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		defer f.Close()
		for b := range w.queue {
			if _, err := f.Write(b); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}()
	return w, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	b := append([]byte(time.Now().Format(time.RFC3339)+" "), p...)
	select {
	case w.queue <- b:
	default:
		_, _ = os.Stderr.Write(b)
	}
	return len(p), nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.queue)
	}
	w.mu.Unlock()
	<-w.done
	return nil
}
