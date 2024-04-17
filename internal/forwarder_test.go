package internal

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testTimeout = 30 * time.Second

func TestForwarder_Forward_Successful(t *testing.T) {
	logger := NewMockLogger(t)
	msgs := []string{"line1", "line2", "line3"}
	reader := strings.NewReader(strings.Join(msgs, "\n"))
	ch := make(chan string)
	logger.On("IsConnected").Return(true)
	logger.On("Disconnect").
		Run(func(mock.Arguments) { close(ch) }).
		Return(nil).Once()
	logger.On("Log", mock.Anything).Run(
		func(args mock.Arguments) {
			ch <- args.Get(0).(string)
		},
	).Times(len(msgs)).Return(nil)
	f := &Forwarder{
		name:   "name",
		bufLen: uint(len(msgs)),
		src:    io.NopCloser(reader),
		logger: logger,
	}
	f.Forward()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, msgs, actualMsgs)
}

func TestForwarder_Forward_Disconnected_ReconnectsButDropsMidMessage(t *testing.T) {
	logger := NewMockLogger(t)
	msgs := []string{"line1", "line2", "line3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	ch := make(chan string)
	logger.On("IsConnected").Return(true).Once()
	logger.On("IsConnected").Return(false).Once()
	logger.On("IsConnected").Return(true).Once()
	logger.On("Disconnect").
		Run(func(mock.Arguments) { close(ch) }).
		Return(nil).Once()
	logger.On("Connect").Return(errors.New("error")).Once()
	logger.On("Log", mock.Anything).
		Run(
			func(args mock.Arguments) {
				ch <- args.Get(0).(string)
			},
		).
		// Log should only be called twice since the second message is dropped.
		Twice().
		Return(nil)
	f := &Forwarder{
		name:   "name",
		bufLen: uint(len(msgs)),
		src:    io.NopCloser(reader),
		logger: logger,
	}
	f.Forward()
	// line2 should be dropped
	expectedMsgs := []string{"line1", "line3"}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, expectedMsgs, actualMsgs)
}

func TestForwarder_Forward_ErrorDuringLogReconnectsOnce(t *testing.T) {
	logger := NewMockLogger(t)
	msgs := []string{"line1", "line2", "line3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	ch := make(chan string)
	logger.On("IsConnected").Return(true).Once()
	logger.On("IsConnected").Return(false).Once()
	logger.On("IsConnected").Return(true).Once()
	logger.On("Connect").Return(nil).Twice()
	logger.On("Disconnect").Return(nil).Once()
	logger.On("Disconnect").
		Run(func(mock.Arguments) { close(ch) }).
		Return(nil).Once()
	logger.On("Log", mock.Anything).Once().Return(errors.New("error"))
	logger.On("Log", mock.Anything).
		Run(
			func(args mock.Arguments) {
				ch <- args.Get(0).(string)
			},
		).
		Times(3).
		Return(nil)
	f := &Forwarder{
		name:   "name",
		bufLen: uint(len(msgs)),
		src:    io.NopCloser(reader),
		logger: logger,
	}
	f.Forward()
	// No messages should be dropped
	expectedMsgs := []string{"line1", "line2", "line3"}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, expectedMsgs, actualMsgs)
}

func TestForwarder_Forward_ErrorsDuringLogDropsMessage(t *testing.T) {
	logger := NewMockLogger(t)
	msgs := []string{"line1", "line2", "line3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	ch := make(chan string)
	logger.On("IsConnected").Return(true).Once()
	logger.On("IsConnected").Return(false).Once()
	logger.On("IsConnected").Return(true).Once()
	logger.On("Connect").Return(nil).Twice()
	logger.On("Disconnect").Return(nil).Twice()
	logger.On("Disconnect").
		Run(func(mock.Arguments) { close(ch) }).
		Return(nil).Once()
	logger.On("Log", mock.Anything).Twice().Return(errors.New("error"))
	logger.On("Log", mock.Anything).
		Run(
			func(args mock.Arguments) {
				ch <- args.Get(0).(string)
			},
		).
		Twice().
		Return(nil)
	f := &Forwarder{
		name:   "name",
		bufLen: uint(len(msgs)),
		src:    io.NopCloser(reader),
		logger: logger,
	}
	f.Forward()
	// line1 should be dropped
	expectedMsgs := []string{"line2", "line3"}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, expectedMsgs, actualMsgs)
}

func TestForwarder_Forward_BufferIsFull_DropsMessages(t *testing.T) {
	logger := NewMockLogger(t)
	msgs := []string{"line1", "line2", "line3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	ch := make(chan string)
	logger.On("IsConnected").Return(true)
	logger.On("Disconnect").
		Run(func(mock.Arguments) { close(ch) }).
		Return(nil)
	logger.On("Log", mock.Anything).
		Run(
			func(args mock.Arguments) {
				ch <- args.Get(0).(string)
			},
		).
		Return(nil)
	f := &Forwarder{
		name: "name",
		// Buffer of 1 causes the last message to drop
		bufLen: 1,
		src:    io.NopCloser(reader),
		logger: logger,
	}
	f.Forward()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.Less(t, len(actualMsgs), len(msgs))
}

func TestNewForwarder_ConnectsLogger(t *testing.T) {
	logger := NewMockLogger(t)
	logger.On("Connect").Return(nil).Once()
	f := NewForwarder("", 0, nil, logger)
	require.NotNil(t, f)
	logger.AssertExpectations(t)
}

func TestNewForwarder_ConnectsLogger_NoError(t *testing.T) {
	logger := NewMockLogger(t)
	logger.On("Connect").Return(errors.New("err")).Once()
	f := NewForwarder("", 0, nil, logger)
	require.NotNil(t, f)
	logger.AssertExpectations(t)
}

func TestForwarder_readLines_Success(t *testing.T) {
	msgs := []string{"1", "2", "3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	f := &Forwarder{name: "dummy", src: io.NopCloser(reader)}
	ch := make(chan string, len(msgs)+1)
	go func() {
		defer close(ch)
		_ = f.readLines(ch)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, msgs, actualMsgs)
}

func TestForwarder_readLines_SuccessNoNewLineAtEOF(t *testing.T) {
	msgs := []string{"1", "2", "3"}
	reader := strings.NewReader(strings.Join(msgs, "\n"))
	f := &Forwarder{name: "dummy", src: io.NopCloser(reader)}
	ch := make(chan string, len(msgs)+1)
	go func() {
		defer close(ch)
		_ = f.readLines(ch)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, msgs, actualMsgs)
}

func TestForwarder_readLines_LargeLine_Success(t *testing.T) {
	msgs := []string{"1", largeString(bufio.MaxScanTokenSize + 1), "2", "3"}
	reader := strings.NewReader(strings.Join(msgs, "\n") + "\n")
	f := &Forwarder{name: "dummy", src: io.NopCloser(reader)}
	ch := make(chan string, len(msgs))
	go func() {
		defer close(ch)
		_ = f.readLines(ch)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	actualMsgs := readChan(ctx, ch)
	require.ElementsMatch(t, msgs, actualMsgs)
}

func largeString(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("a")
	}
	return b.String()
}

func readChan(ctx context.Context, ch <-chan string) []string {
	msgs := make([]string, 0)
	for {
		select {
		case <-ctx.Done():
			return msgs
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		}
	}
}
