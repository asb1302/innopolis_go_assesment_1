APP_NAME = innopolis_go_assesment_1

BIN_DIR = cmd/web

TEST_DIR = internal/app

SRC_DIR = cmd/web

GO = go
GOFLAGS = -v

default: test

test:
	$(GO) test $(GOFLAGS) ./$(TEST_DIR)/...

build-mac:
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BIN_DIR)/$(APP_NAME)-mac $(SRC_DIR)/main.go

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN_DIR)/$(APP_NAME)-linux $(SRC_DIR)/main.go

run-mac: build-mac
	./$(BIN_DIR)/$(APP_NAME)-mac

run-linux: build-linux
	./$(BIN_DIR)/$(APP_NAME)-linux

clean:
	rm -f $(BIN_DIR)/$(APP_NAME)-mac
	rm -f $(BIN_DIR)/$(APP_NAME)-linux

.PHONY: default test build-mac build-linux run-mac run-linux clean
