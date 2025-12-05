# Binary filename
BINARY_FILE=gh-runner
# Binary directory
BIN_DIR=bin

install:
	/bin/bash scripts/install_podman.sh

build: $(BIN_DIR) build-ct-image
	go build -o $(BIN_DIR)/$(BINARY_FILE) server/server.go server/config.go server/logger.go server/utils.go

# Create binary dir if not exists
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build-ct-image:
	/bin/bash scripts/build_container_image.sh

run: build
	./$(BIN_DIR)/$(BINARY_FILE)

clean:
	rm -rf ./bin