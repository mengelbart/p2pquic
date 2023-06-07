package p2pquic

import (
	"io"
	"log"
)

// A wrapper for io.Writer that also logs the message.
type loggingWriter struct{ io.Writer }

func (w loggingWriter) Write(b []byte) (int, error) {
	log.Printf("Server: Got '%s'\n", string(b))
	return w.Writer.Write(b)
}
