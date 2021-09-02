.PHONY: build run

build:
	GOOS=$(os) GOARCH=$(arch) go build -o bot ./cmd/bot.go

run:
	go run cmd/bot.go
