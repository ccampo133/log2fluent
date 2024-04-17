FROM golang:1.22 AS build

# Set destination for COPY.
WORKDIR /app

# Download dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code.
COPY . .

# Build.
RUN CGO_ENABLED=0 go build \
    # TODO: use a build arg for the version
    -ldflags="-X main.version=$(git describe --tags --always)" \
    -o log2fluent main.go

FROM scratch

COPY --from=build /app/log2fluent /log2fluent

ENTRYPOINT ["/log2fluent"]
