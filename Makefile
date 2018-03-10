# The binary to build (just the basename).
BIN := dash2alexa

VERSION := $(shell git describe --tags --always --dirty)

all: build

.PHONY: build

build: $(BIN)
	go build -o $(BIN) dash2alexa.go

run: build
	sudo ./dash2alexa

version:
	@echo $(VERSION)

clean:
	rm $(BIN)





