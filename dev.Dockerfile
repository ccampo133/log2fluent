# This Dockerfile can be used for local development purposes. The Dockerfile
# of the 'production' image is just called 'Dockerfile' and is used by
# goreleaser.
FROM golang:1.22 AS build

ARG VERSION=dev

# Set destination for COPY.
WORKDIR /app

# Download dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code.
COPY . .

# Build.
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=$VERSION" \
    -o log2fluent main.go

FROM scratch

COPY --from=build /app/log2fluent /log2fluent

ENTRYPOINT ["/log2fluent"]
