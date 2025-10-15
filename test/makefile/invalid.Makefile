# Invalid Makefile with syntax errors
.PHONY: all clean test

all: build test

build
	@echo "Building project..."  # Missing colon after target
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
