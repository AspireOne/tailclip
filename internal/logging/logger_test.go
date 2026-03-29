package logging

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewLogWriterKeepsPrimaryLoggingWhenMirrorFails(t *testing.T) {
	var primary bytes.Buffer
	writer := newLogWriter(&primary, failingWriter{err: errors.New("stdout unavailable")})

	n, err := writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("expected write to succeed, got %v", err)
	}
	if n != len("hello") {
		t.Fatalf("expected %d bytes written, got %d", len("hello"), n)
	}
	if got := primary.String(); got != "hello" {
		t.Fatalf("expected primary writer to receive log entry, got %q", got)
	}
}

func TestNewLogWriterReturnsPrimaryError(t *testing.T) {
	wantErr := errors.New("disk full")
	writer := newLogWriter(failingWriter{err: wantErr})

	if _, err := writer.Write([]byte("hello")); !errors.Is(err, wantErr) {
		t.Fatalf("expected primary writer error %v, got %v", wantErr, err)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}
