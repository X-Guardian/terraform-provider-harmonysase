default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	terraform fmt -recursive ./examples/
	cd tools && go tool tfplugindocs generate --provider-dir .. -provider-name harmonysase
	./scripts/set-subcategories.sh

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

.PHONY: fmt lint test build install generate
