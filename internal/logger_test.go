package internal

import (
	"errors"
	"testing"

	"github.com/IBM/fluent-forward-go/fluent/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockMessageClient struct {
	client.MessageClient
	mock.Mock
}

func (m *mockMessageClient) Reconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockMessageClient) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockMessageClient) SendMessage(tag string, record any) error {
	args := m.Called(tag, record)
	return args.Error(0)
}

func TestFluentLogger_Log(t *testing.T) {
	type fields struct {
		tag   string
		src   string
		extra map[string]string
		c     *mockMessageClient
	}
	tests := []struct {
		name            string
		fields          fields
		msg             string
		wantErr         require.ErrorAssertionFunc
		otherAssertions func(t *testing.T, logger *FluentLogger)
	}{
		{
			name: "log is successful w/ correct tag and stream",
			msg:  "hello",
			fields: func() fields {
				tag := "tag"
				src := "stream"
				c := new(mockMessageClient)
				rec := map[string]string{"log": "hello", "stream": src}
				c.On("SendMessage", tag, rec).Return(nil)
				return fields{tag: tag, src: src, c: c}
			}(),
			wantErr: require.NoError,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
		{
			name: "log is successful w/ correct tag, stream, and extra",
			msg:  "hello",
			fields: func() fields {
				tag := "tag"
				stream := "stream"
				extra := map[string]string{"foo": "bar"}
				c := new(mockMessageClient)
				rec := map[string]string{"log": "hello", "stream": stream, "foo": "bar"}
				c.On("SendMessage", tag, rec).Return(nil)
				return fields{tag: tag, src: stream, extra: extra, c: c}
			}(),
			wantErr: require.NoError,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
		{
			name: "log error",
			fields: func() fields {
				c := new(mockMessageClient)
				c.On("SendMessage", mock.Anything, mock.Anything).Return(errors.New("error"))
				return fields{c: c}
			}(),
			wantErr: require.Error,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				logger := &FluentLogger{
					tag:    tt.fields.tag,
					stream: tt.fields.src,
					extra:  tt.fields.extra,
					c:      tt.fields.c,
				}
				tt.wantErr(t, logger.Log(tt.msg))
				tt.fields.c.AssertExpectations(t)
				if tt.otherAssertions != nil {
					tt.otherAssertions(t, logger)
				}
			},
		)
	}
}

func TestFluentLogger_Connect(t *testing.T) {
	tests := []struct {
		name            string
		c               *mockMessageClient
		wantErr         require.ErrorAssertionFunc
		otherAssertions func(t *testing.T, logger *FluentLogger)
	}{
		{
			name: "connect is successful",
			c: func() *mockMessageClient {
				c := new(mockMessageClient)
				c.On("Reconnect").Return(nil)
				return c
			}(),
			wantErr: require.NoError,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.True(t, logger.connected)
			},
		},
		{
			name: "connect error",
			c: func() *mockMessageClient {
				c := new(mockMessageClient)
				c.On("Reconnect").Return(errors.New("error"))
				return c
			}(),
			wantErr: require.Error,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				logger := &FluentLogger{c: tt.c}
				tt.wantErr(t, logger.Connect())
				tt.c.AssertExpectations(t)
				if tt.otherAssertions != nil {
					tt.otherAssertions(t, logger)
				}
			},
		)
	}
}

func TestFluentLogger_Disconnect(t *testing.T) {
	tests := []struct {
		name            string
		c               *mockMessageClient
		connected       bool
		wantErr         require.ErrorAssertionFunc
		otherAssertions func(t *testing.T, logger *FluentLogger)
	}{
		{
			name: "disconnect is successful",
			c: func() *mockMessageClient {
				c := new(mockMessageClient)
				c.On("Disconnect").Return(nil)
				return c
			}(),
			wantErr: require.NoError,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
		{
			name: "disconnect error",
			c: func() *mockMessageClient {
				c := new(mockMessageClient)
				c.On("Disconnect").Return(errors.New("error"))
				return c
			}(),
			wantErr: require.Error,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
		{
			name: "disconnect sets connected to false",
			c: func() *mockMessageClient {
				c := new(mockMessageClient)
				c.On("Disconnect").Return(nil)
				return c
			}(),
			connected: true,
			wantErr:   require.NoError,
			otherAssertions: func(t *testing.T, logger *FluentLogger) {
				require.False(t, logger.connected)
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				logger := &FluentLogger{c: tt.c, connected: tt.connected}
				tt.wantErr(t, logger.Disconnect())
				tt.c.AssertExpectations(t)
				if tt.otherAssertions != nil {
					tt.otherAssertions(t, logger)
				}
			},
		)
	}
}

func TestFluentLogger_IsConnected(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		want      bool
	}{
		{
			name:      "is connected",
			connected: false,
			want:      false,
		},
		{
			name:      "is not connected",
			connected: true,
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				logger := &FluentLogger{connected: tt.connected}
				require.Equal(t, tt.want, logger.IsConnected())
			},
		)
	}
}

func TestNewFluentLogger_LoggerIsNotConnected(t *testing.T) {
	l := NewFluentLogger("", "", "", "", nil)
	require.False(t, l.IsConnected())
}
