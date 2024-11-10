.PHONY: build

build:
	go build -o bin/machine cmd/main.go

clean:
	rm bin/machine

download:
	./scripts/download-fc.sh
	./scripts/download-kernel.sh
