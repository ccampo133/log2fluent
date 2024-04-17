package internal

import (
	"fmt"

	"github.com/IBM/fluent-forward-go/fluent/client"
)

// Logger is the interface that represents a type that writes log messages to
// some (remote) destination which requires a connection.
//
//go:generate mockery --inpackage --name=Logger --filename mock_logger.go
type Logger interface {
	// Log writes the given message to the destination.
	Log(msg string) error
	// Connect establishes the Logger's connection to the destination.
	Connect() error
	// Disconnect breaks the connection to the destination. If there is no
	// connection, Disconnect should be a no-op.
	Disconnect() error
	// IsConnected returns true if the Logger has an established connection.
	IsConnected() bool
}

// FluentLogger is an implementation of Logger that writes a given
// string to a configured fluent address. It is not thread safe.
type FluentLogger struct {
	tag, stream string
	extra       map[string]string
	c           client.MessageClient
	connected   bool
}

// NewFluentLogger instantiates a new FluentLogger. Note that it does not
// automatically connect the logger. Therefore, FluentLogger.Connect should be
// called before any calls to FluentLogger.Log.
func NewFluentLogger(network, addr, tag, stream string, extra map[string]string) *FluentLogger {
	return &FluentLogger{
		tag:    tag,
		stream: stream,
		extra:  extra,
		c: client.New(
			client.ConnectionOptions{
				Factory: &client.ConnFactory{
					Network: network,
					Address: addr,
				},
			},
		),
	}
}

// Log sends a given string as a message to the fluent address this logger is
// connected to. If the logger is not connected for some reason, call Connect
// first.
func (w *FluentLogger) Log(msg string) error {
	record := map[string]string{
		"log":    msg,
		"stream": w.stream,
	}
	for k, v := range w.extra {
		record[k] = v
	}
	return w.c.SendMessage(w.tag, record)
}

func (w *FluentLogger) Connect() error {
	if err := w.c.Reconnect(); err != nil {
		w.connected = false
		return fmt.Errorf("error connecting logger: %w", err)
	}
	w.connected = true
	return nil
}

func (w *FluentLogger) Disconnect() error {
	w.connected = false
	return w.c.Disconnect()
}

func (w *FluentLogger) IsConnected() bool {
	return w.connected
}
