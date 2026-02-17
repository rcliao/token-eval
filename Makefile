.PHONY: build test vet clean acceptance all

build:
	@mkdir -p bin
	go build -o bin/token-eval ./cmd/token-eval

test:
	go test ./... -v

vet:
	go vet ./...

clean:
	rm -rf bin

acceptance: build
	bash test/acceptance.sh

all: vet test build acceptance
