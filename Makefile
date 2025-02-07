
# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
BINARY_NAME=machine

# Targets
.PHONY: all test build release clean

all: test build

test:
	$(GOTEST) -v ./...

build:
	$(GOBUILD) -o $(BINARY_NAME) .

release: clean
	$(GOBUILD) -o $(BINARY_NAME) .

clean:
	if [ -f $(BINARY_NAME) ]; then rm $(BINARY_NAME); fi

fmt:
	$(GOFMT) ./...
