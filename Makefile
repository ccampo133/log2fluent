tidy:
	go mod tidy

bump-deps:
	go get -u -t ./...

generate:
	go generate ./...

lint:
	golangci-lint run

# Using --count=1 disables test caching
test:
	go test -v -race ./... --count=1

integration-test:
	go test -v -race ./... --count=1 --tags=integration

clean:
	go clean -i ./...

build: clean tidy
	go build -o log2fluent

docker-build:
	docker build -f dev.Dockerfile -t log2fluent .
