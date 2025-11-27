# Binary filename
BINARY_FILE=gh-runner
# Binary directory
BIN_DIR=bin

# ENV VAR required for run

build: $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_FILE) server/server.go server/config.go

# Create binary dir if not exists
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

run: build
	./$(BIN_DIR)/$(BINARY_FILE)

clean:
	rm -rf ./bin