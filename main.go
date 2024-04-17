package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ccampo133/log2fluent/internal"
)

func usage() {
	_, _ = fmt.Fprint(
		flag.CommandLine.Output(),
		`Usage: log2fluent [options] <command> [args...]

Execute a command, capture its stdout and/or stderr logs, and forward those logs
to Fluent via the Fluent Forward Protocol.

Supported options:
`,
	)
	flag.PrintDefaults()
}

// The application version. It is intended to be set at compile time via the
// linker (e.g. -ldflags="-X main.version=...").
var version = "latest"

func main() {
	var (
		tag              string
		outDest, errDest string
		outPipe, errPipe *pipe
		extraAttrs       string
		bufLen           uint
		debugEnabled     bool
		printVersion     bool
		fwdrs            []*internal.Forwarder
	)
	flag.StringVar(
		&tag,
		"tag",
		"",
		"the log identifier, e.g. service name, container ID, etc.",
	)
	flag.StringVar(
		&outDest,
		"stdout",
		"",
		"fluent-bit address for forwarding stdout ([network://]addr).",
	)
	flag.StringVar(
		&errDest,
		"stderr",
		"",
		"fluent-bit address for forwarding stderr ([network://]addr).",
	)
	flag.UintVar(
		&bufLen,
		"buflen",
		8192,
		"message buffer length, i.e. the number of messages buffered before being\ndropped.",
	)
	flag.StringVar(
		&extraAttrs,
		"extra",
		"",
		"comma separated list of extra key/value attributes to add to log\nmessages, e.g. key1=val1,key2=val2.",
	)
	flag.BoolVar(
		&debugEnabled,
		"debug",
		false,
		"enable debug logging.",
	)
	flag.BoolVar(
		&printVersion,
		"version",
		false,
		"print the version and exit.",
	)
	flag.Usage = usage
	flag.Parse()

	if printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	logLevel := slog.LevelInfo
	if debugEnabled {
		logLevel = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))

	extra := parseExtraAttrs(extraAttrs)

	// Create pipes for child process's standard streams.
	stdout := os.Stdout
	if outDest != "" {
		var fwd *internal.Forwarder
		outPipe, fwd = newPipeAndForwarder("stdout", outDest, tag, bufLen, extra)
		fwdrs = append(fwdrs, fwd)
		stdout = outPipe.writeFd
	}
	stderr := os.Stderr
	if errDest != "" {
		var fwd *internal.Forwarder
		errPipe, fwd = newPipeAndForwarder("stderr", errDest, tag, bufLen, extra)
		fwdrs = append(fwdrs, fwd)
		stderr = errPipe.writeFd
	}

	// Start child process.
	cwd, err := os.Getwd()
	if err != nil {
		logFatal("error getting working directory: %v", err)
	}
	attr := os.ProcAttr{
		Dir:   cwd,
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, stdout, stderr},
	}
	child, err := os.StartProcess(flag.Arg(0), flag.Args(), &attr)
	if err != nil {
		logFatal("error executing %s: %v", flag.Arg(0), err)
	}
	// Close write file descriptors in the parent process.
	if outPipe != nil {
		_ = outPipe.writeFd.Close()
	}
	if errPipe != nil {
		_ = errPipe.writeFd.Close()
	}

	// Start forwarding logs to Fluent.
	for _, fwdr := range fwdrs {
		fwdr.Forward()
	}

	// Wait for child process to exit.
	state, err := child.Wait()
	if err != nil {
		logFatal("error waiting for child process", "error", err)
	}
	if !state.Exited() {
		// Child process terminated due to a signal.
		slog.Info("child process terminated due to signal", "signal", state.String())
		os.Exit(254)
	}
	// Done - exit with the exit code of the child process.
	os.Exit(state.ExitCode())
}

func logFatal(format string, args ...any) {
	slog.Error(format, args...)
	os.Exit(1)
}

func parseExtraAttrs(s string) map[string]string {
	entries := strings.Split(s, ",")
	extra := make(map[string]string)
	for _, e := range entries {
		parts := strings.Split(strings.TrimSpace(e), "=")
		if len(parts) == 2 {
			extra[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return extra
}

func parseLocation(loc string) (string, string) {
	var network, addr string
	parts := strings.SplitN(loc, "://", 2)
	if len(parts) == 2 {
		network = parts[0]
		addr = parts[1]
	} else {
		network = "tcp"
		addr = loc
	}
	return network, addr
}

type pipe struct {
	readFd, writeFd *os.File
}

func newPipe() (*pipe, error) {
	readFd, writeFd, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &pipe{readFd: readFd, writeFd: writeFd}, nil
}

func newPipeAndForwarder(stream, dest, tag string, bufLen uint, extra map[string]string) (
	*pipe,
	*internal.Forwarder,
) {
	p, err := newPipe()
	if err != nil {
		logFatal("error creating pipe: %v", err)
	}
	network, addr := parseLocation(dest)
	if tag == "" {
		tag = stream
	}
	logger := internal.NewFluentLogger(network, addr, tag, stream, extra)
	fwd := internal.NewForwarder(stream, bufLen, p.readFd, logger)
	return p, fwd
}
