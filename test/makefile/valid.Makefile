# Sample Makefile for testing
.PHONY: all clean test

all: build test

build:
	@echo "Building project..."
	gcc -o main main.c

test:
	@echo "Running tests..."
	./test_runner

clean:
	@echo "Cleaning up..."
	rm -f main *.o

install: build
	@echo "Installing..."
	cp main /usr/local/bin/
