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
$ log2fluent \
  # Fluent tag to use for log messages.
  -tag="yourapp" \
  # Send yourapp's stdout to Fluent via a Unix domain socket.
  -stdout=unix:///path/to/fluent.sock \
  # Send yourapp's stderr to Fluent via a TCP socket.
  -stderr=tcp://fluent.example.com:24224 \
  /path/to/yourapp
```

To see all available options, run `log2fluent` without any arguments.

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
checksums are also signed with my GPG key (fingerprint `TODO`),
and you can verify the checksums file, e.g.:

```bash
# Replace <version> with the desired version, e.g. v0.1.0.
# The below commands assume that you are in the same directory as the binary and
# checksums/signature.
sha256sum -c log2fluent_<version>_checksums.txt
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

CMD ["/usr/local/bin/log2fluent", ..., "/path/to/yourapp", ...]
```

You can run the image directly too:

```bash
docker run --rm ghcr.io/ccampo133/log2fluent:latest
```

Tags for each version of `log2fluent` are released, as well as a `latest` tag.
