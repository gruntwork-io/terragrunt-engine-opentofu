default: build

build: $(shell find . \( -type f -name '*.go' -print \))
	set -xe ;\
	go build -o engine -ldflags "-extldflags '-static'" .

clean:
	rm -f engine

lint:
	golangci-lint run ./...

test:
	go test ./...

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2

.PHONY: default lint protoc test tools
