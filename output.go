package goyek

import (
	"fmt"
	"io"
	"sync"
)

func writeLinef(w io.Writer, format string, a ...interface{}) {
	_, _ = fmt.Fprintf(w, format+"\n", a...)
}

// Output contains the writers to communicate results.
type Output struct {
	// Standard output is for information that is expected from the executed tasks.
	Standard io.Writer
	// Messaging output is for any status information, such as logging or error messages.
	Messaging io.Writer
}

// WriteMessagef prints the given format message and its arguments to the Messaging writer.
// The result of this action is ignored.
func (out Output) WriteMessagef(format string, a ...interface{}) {
	writeLinef(out.Messaging, format, a...)
}

// bufferedOutput stores all written data in memory.
type bufferedOutput struct {
	mutex   sync.Mutex
	entries []*bufferedEntry
}

type bufferedEntry struct {
	standard bool
	data     []byte
}

// Output returns an Output instance with writers that store written data in this buffer.
func (buffer *bufferedOutput) Output() Output {
	return Output{
		Standard:  &bufferedWriter{buffer: buffer, standard: true},
		Messaging: &bufferedWriter{buffer: buffer, standard: false},
	}
}

// WriteTo will reproduce the buffered information into the provided output.
// The buffer keeps track when which writer was used previously and it
// will reproduce the same sequence to the provided output.
func (buffer *bufferedOutput) WriteTo(other Output) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	currentEntries := buffer.entries
	for _, entry := range currentEntries {
		w := other.Messaging
		if entry.standard {
			w = other.Standard
		}
		_, _ = w.Write(entry.data)
	}
}

func (buffer *bufferedOutput) add(entry *bufferedEntry) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	buffer.entries = append(buffer.entries, entry)
}

type bufferedWriter struct {
	buffer   *bufferedOutput
	standard bool
}

// Write copies the provided data into the backing buffer.
func (w bufferedWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	entry := &bufferedEntry{
		standard: w.standard,
		data:     make([]byte, n),
	}
	copy(entry.data, p)
	w.buffer.add(entry)
	return
}
