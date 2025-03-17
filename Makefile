GO=go
BUILD_DIR=bin
MAIN_BINARY=machine
MKEXT4_BINARY=mkext4

.PHONY: all clean build build-main build-ext4 test fmt install run

all: build

build: build-main build-ext4

build-main: $(BUILD_DIR)/$(MAIN_BINARY)

build-ext4: $(BUILD_DIR)/$(MKEXT4_BINARY)

$(BUILD_DIR)/$(MAIN_BINARY):
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $@ .

$(BUILD_DIR)/$(MKEXT4_BINARY):
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/mkext4

test:
	$(GO) test -v ./...

fmt:
	$(GO) fmt ./...

clean:
	@rm -f ./$(BUILD_DIR)/$(MAIN_BINARY)
	@rm -f ./$(BUILD_DIR)/$(MKEXT4_BINARY)

install: build
	install -m 755 $(BUILD_DIR)/$(MKEXT4_BINARY) /usr/local/bin/$(MKEXT4_BINARY)

run: build
	./$(BUILD_DIR)/$(MAIN_BINARY)