package internal

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Forwarder forwards messages from some source reader (typically a read-only
// fd from an os.Pipe) to some destination fluentWriter.
type Forwarder struct {
	name   string        // The forwarder's name, e.g. "stdout" or "stderr".
	bufLen uint          // The message channel buffer length.
	src    io.ReadCloser // Where we read the logs from.
	logger Logger        // Where we send the logs to.
}

// NewForwarder returns a new Forwarder based on an input stream and a fluent
// destination. If there is an error connecting to the fluent destination, no
// error is returned and Forwarder.writer is set to nil. Then the connection
// will be retried on each successive call for Forwarder.Forward, and
// Forwarder.writer will be set once the connection is successful.
func NewForwarder(name string, bufLen uint, src io.ReadCloser, logger Logger) *Forwarder {
	if err := logger.Connect(); err != nil {
		slog.Debug("error connecting logger; will be retried on first message", "name", name, "error", err)
	}
	return &Forwarder{name: name, bufLen: bufLen, src: src, logger: logger}
}

// Forward forwards log messages by launching two goroutines - one to read
// messages (line by line) from the configured reader, and one to write
// messages to the configured Logger. It returns immediately after launching
// these goroutines. The reader goroutine passes messages to the writer
// goroutine via a buffered channel. The channel's buffer length is determined
// by the Forwarder's bufLen property. Note that if the buffer is full, messages
// are unceremoniously dropped. Also note that if the underlying logger
// connection is not established, the Logger connection will be retried on each
// message until it can be successfully established. If the connection can't be
// established while processing a particular message, that message will be
// dropped. If there is an error during the Logger.Log call, it will be ignored
// (but the error will be writen to stderr) and the log message will likely be
// lost as well. Additionally, in the case of errors to Logger.Log, the Logger's
// connection is explicitly disconnected and retried on the next message for
// resiliency.
func (f *Forwarder) Forward() {
	msgs := make(chan string, f.bufLen)

	// Reader
	go func(msgs chan<- string) {
		// When readLines returns (due to either EOF or an error) and this
		// goroutine exits, the msgs channel is closed, which will cause the
		// writer goroutine to exit as well.
		defer func() {
			close(msgs)
			_ = f.src.Close()
		}()
		if err := f.readLines(msgs); err != nil {
			// Nothing we can really do here but log this and quit.
			slog.Error("error reading lines", "name", f.name, "error", err)
		}
	}(msgs)

	// Writer
	go func(msgs <-chan string) {
		defer func() { _ = f.logger.Disconnect() }()
		for msg := range msgs {
			if !f.logger.IsConnected() {
				// Try establishing a connection.
				if err := f.logger.Connect(); err != nil {
					slog.Debug("error connecting logger; dropping msg", "name", f.name, "error", err)
					continue
				}
				slog.Debug("logger reconnected", "name", f.name)
			}
			if err := f.logger.Log(msg); err != nil {
				// Probably lost connection, try to reconnect once and re-send
				// the message... otherwise drop it.
				_ = f.logger.Disconnect()
				if err := f.logger.Connect(); err != nil {
					slog.Debug("error connecting logger; dropping msg", "name", f.name, "error", err)
					continue
				}
				slog.Debug("logger reconnected", "name", f.name)
				if err := f.logger.Log(msg); err != nil {
					// Still can't log; will retry on next message.
					slog.Error("error logging msg; dropping msg", "name", f.name, "error", err)
					_ = f.logger.Disconnect()
				}
			}
		}
	}(msgs)
}

// readLines reads lines from the Forwarder's reader, and passes them to the
// provided message channel until there is no more input available from the
// reader (EOF). Lines may be arbitrarily long. If the channel's buffer is
// full, the line is dropped. If there is an error reading from the reader at
// any point, the error is returned.
func (f *Forwarder) readLines(msgs chan<- string) error {
	reader := bufio.NewReader(f.src)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error reading from reader: %w", err)
			}
			// It's common to have a blank line at the end of the file, which we
			// don't care about and can safely ignore.
			if line == "" {
				return nil
			}
		}
		select {
		case msgs <- strings.TrimSuffix(line, "\n"):
		default:
			// We're running behind - drop the message.
			slog.Debug("message channel buffer is full; dropping msg", "name", f.name)
		}
		// Reached EOF but still had a message to send. We're done now.
		if err == io.EOF {
			return nil
		}
	}
}
