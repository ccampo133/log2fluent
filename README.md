# log2fluent

`log2fluent` is a command line tool that sends application logs to a Fluent
server (e.g. [Fluent Bit](https://fluentbit.io/),
[Fluentd](https://www.fluentd.org/)) via the
[Fluent Forward Protocol](https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1)
as structured log data.

Adopted from my blog original post:
["Utilizing Pipes for High Performance Log Management"](https://cyral.com/blog/utilizing-pipes-for-high-performance-log-management/).

## Motivation

It is pretty common to use Fluent Bit or Fluentd as a log aggregator. The
typical way to do this is to have applications write logs to a file and have
Fluent directly read from that file and forward the logs to some log management
platform (Splunk, Elastic, Loki, Graylog etc.).

This is a simple and effective way to collect logs, but it has some baggage such
as potential disk I/O overhead, log rotation requirements, etc. In some
(admittedly rare) scenarios, such as those with limited resources or with
ultra-high-throughput requirements, it may be beneficial to avoid writing logs
to disk and instead stream them directly to fluent.

If you're using Docker, the first approach I would recommend is to use Docker's
native logging driver for
[Fluentd](https://docs.docker.com/engine/logging/drivers/fluentd/). If you're
not using Docker, or for some reason you can't or don't want to use that log
driver, but still want to send your logs to Fluent without writing to disk, then
keep reading -`log2fluent` may be useful to you.

## Overview

`log2fluent` is a simple command line tool that runs your app as a child process
and redirects its output streams into a pair of pipes. `log2fluent` then reads
from those pipes and sends the content to a Fluent server (e.g.
[Fluent Bit](https://fluentbit.io/) or [Fluentd](https://www.fluentd.org/)) via
the
[Fluent Forward Protocol](https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1)
transported over a TCP or Unix domain socket connection.

The most common use-case is to avoid writing logs to disk in any capacity, such
as when you're running in a system with limited I/O and/or storage resources
and high throughput requirements.

Output from the standard streams is read line-by-line, and each line is sent as
a separate log message. To avoid backpressure on the write side (the child app),
`log2fluent` will buffer log messages up to a certain message count before
dropping them entirely. This is to prevent the child app from blocking on writes
to the pipe if the Fluent server is slow or unresponsive. Support for different
backpressure strategies (as opposed to dropping messages outright) may be added
in the future.

Logs are sent to Fluent as structured messages with the following keys:

* `log`: Contains the log message itself.
* `stream`: The name of the stream where the message originated from - either
  `stdout` or `stderr`.

## Usage

Assuming you have an application called `yourapp` that writes logs to stdout and
stderr, you can use `log2fluent` to send those logs to a Fluent server. For
example:

```bash
log2fluent \
  # Fluent tag to use for log messages.
  -tag="yourapp" \
  # Send yourapp's stdout to Fluent via a Unix domain socket.
  -stdout=unix:///path/to/fluent.sock \
  # Send yourapp's stderr to Fluent via a TCP socket.
  -stderr=tcp://fluent.example.com:24224 \
  /path/to/yourapp
```

To see all available options, run `log2fluent` without any arguments.

### Example with Fluent Bit

Here's a simple example of how you might use `log2fluent` with Fluent Bit.
Consider the following Fluent Bit configuration (`fluent-bit.conf`):

```conf
[INPUT]
    Name        forward
    Unix_Path   stdout.sock
    Tag_Prefix  stdout

[INPUT]
    Name        forward
    Listen      0.0.0.0
    Port        24224
    Tag_Prefix  stderr

[OUTPUT]
    Name   stdout
    Match  *
```

This configuration sets up two
[`forward`](https://docs.fluentbit.io/manual/pipeline/inputs/forward) inputs:

1. A Unix domain socket at `stdout.sock` (relative to the directory where
   Fluent Bit is run from) that listens for log messages and prefixes their
   incoming tag prefix `stdout`.
2. A TCP socket that listens on `0.0.0.0:24224` and prefixes incoming log
   messages with `stderr`.

The idea behind this configuration is to separate and distinguish logs from
`stdout` and `stderr`.

It also establishes a single output that simply writes all incoming log messages
Fluent Bit's stdout.

For this example, create the above configuration in a file called
`fluent-bit.conf` in some directory. You can then run Fluent Bit in a terminal
with this configuration like so:

```bash
fluent-bit --quiet -c fluent-bit.conf
```

Next, create a small shell script called `hello.sh` to simulate an application
that writes logs to `stdout` and `stderr`:

```bash
#!/bin/sh

echo "Hello, stdout!"
echo "Hello, stderr!" >&2
```

Make sure to make `hello.sh` executable:

```bash
chmod +x hello.sh
```

From the same directory, now run `log2fluent` with the following command in
another terminal:

```bash
log2fluent \
  -tag="hello" \
  -stdout=unix://$(pwd)/stdout.sock \
  -stderr=tcp://localhost:24224 \
  ./hello.sh
```

You should see the log messages from `hello.sh` in the terminal where Fluent Bit
is running, e.g.:

```bash
[0] stdouthello: [[1630425600.000000000, {}], {"log"=>"Hello, stdout!", "stream"=>"stdout"}]
[0] stderrhello: [[1630425600.000000000, {}], {"log"=>"Hello, stderr!", "stream"=>"stderr"}]
```

You could take this example further by matching `stdout` and `stderr` logs
separately and routing them to different outputs, e.g.:

```conf
# INPUTs omitted for brevity...

# Send stdout logs to an S3 bucket.
[OUTPUT]
    Name    s3
    Match   stdout*
    Bucket  my-bucket
    Region  us-west-2

# Send stderr logs to Elasticsearch.
[OUTPUT]
    Name   es
    Match  stderr*
    Host   es.example.com
    Port   9200
```

Feel free to play with this example and experiment with different configurations
of inputs, filters, and outputs.

## Installation

### From Source

```bash
go install github.com/ccampo133/log2fluent
```

### Release Artifacts

Download the latest release zip for your platform from the
[releases page](https://github.com/ccampo133/log2fluent/releases) and extract
the binary to a location in your `PATH`.

```bash
unzip log2fluent_*.zip
mv log2fluent /usr/local/bin
```

The SHA256 checksums for each release are provided in a file named
`log2fluent_<version>_checksums.txt`. You can verify the integrity of the
downloaded binary by comparing its checksum to the one in the file. The
checksums are also signed with [my GPG key](https://github.com/ccampo133.gpg),
and you can verify the checksums file, e.g.:

```bash
# Replace <version> with the desired version without the 'v' prefix, e.g. 0.1.0.
# The below commands assume that you are in the same directory as the binary and
# checksums/signature.
sha256sum -c log2fluent_<version>_checksums.txt
# First import my GPG key if you haven't already:
# curl https://github.com/ccampo133.gpg | gpg --import
gpg --verify log2fluent_<version>_checksums.txt.sig log2fluent_<version>_checksums.txt
````

### Docker

It doesn't make much sense to run this in a container, since the child process
will also need to be in the container, but an image is available for convenience
anyway. For example, maybe you want to incorporate `log2fluent` in your own
image via a multi-stage build `COPY --from` directive, e.g.:

```dockerfile
FROM ghcr.io/ccampo133/log2fluent:latest as l2f
FROM scratch

COPY --from=l2f /log2fluent /usr/local/bin/log2fluent

# ...do other stuff...

CMD ["/usr/local/bin/log2fluent", ...options..., "/path/to/yourapp", ...]
```

You can run the image directly too:

```bash
docker run --rm ghcr.io/ccampo133/log2fluent:latest
```

Tags for each version of `log2fluent` are released, as well as a `latest` tag.

## Development

There is a [`Makefile`](Makefile) with some common development tasks. Please see
the file for more information. It's a pretty standard Go project - there's not
much to it.

To build (requires Go 1.22+):

```bash
make build
```

To run tests:

```bash
make test
```

To build a local Docker image called `log2fluent`:
```bash
make docker-build
```
