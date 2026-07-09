default:
    just --list

build:
    go build -o bin/lazylore.exe ./cmd/lazylore

run: build
    ./bin/lazylore.exe

unit-test:
    go test ./...

test: unit-test

format:
    gofmt -l -w .
