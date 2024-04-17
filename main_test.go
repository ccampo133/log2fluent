package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseExtraAttrs(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want map[string]string
	}{
		{
			name: "empty string",
			s:    "",
			want: map[string]string{},
		},
		{
			name: "non comma separated string",
			s:    "foo",
			want: map[string]string{},
		},
		{
			name: "comma separated, but not k/v",
			s:    "foo,bar,baz",
			want: map[string]string{},
		},
		{
			name: "comma separated and k/v",
			s:    "foo=bar,baz=qux",
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
		},
		{
			name: "comma separated and k/v with non k/v in between",
			s:    "foo=bar,baz,qux=quxx",
			want: map[string]string{
				"foo": "bar",
				"qux": "quxx",
			},
		},
		{
			name: "comma separated k/v with random whitespace",
			s:    " foo= bar, baz=qux,   a=   b ,c=d",
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
				"a":   "b",
				"c":   "d",
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				require.Equal(t, tt.want, parseExtraAttrs(tt.s))
			},
		)
	}
}

func Test_parseLocation(t *testing.T) {
	tests := []struct {
		name        string
		loc         string
		wantNetwork string
		wantAddr    string
	}{
		{
			name:        "default to tcp",
			loc:         "localhost:1234",
			wantNetwork: "tcp",
			wantAddr:    "localhost:1234",
		},
		{
			name:        "udp",
			loc:         "udp://localhost:1234",
			wantNetwork: "udp",
			wantAddr:    "localhost:1234",
		},
		{
			name:        "unix",
			loc:         "unix:///path/to/sock",
			wantNetwork: "unix",
			wantAddr:    "/path/to/sock",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				gotNetwork, gotAddr := parseLocation(tt.loc)
				require.Equal(t, tt.wantNetwork, gotNetwork)
				require.Equal(t, tt.wantAddr, gotAddr)
			},
		)
	}
}
