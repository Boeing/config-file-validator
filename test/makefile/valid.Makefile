# Valid Makefile for testing
.PHONY: all clean test install

# Variables
CC = gcc
CFLAGS = -Wall -Wextra -std=c99
TARGET = main
SOURCES = main.c utils.c

# Default target
all: $(TARGET)

# Build target
$(TARGET): $(SOURCES)
	$(CC) $(CFLAGS) -o $(TARGET) $(SOURCES)

# Test target
test: $(TARGET)
	./$(TARGET) --test

# Clean target
clean:
	rm -f $(TARGET) *.o

# Install target
install: $(TARGET)
	cp $(TARGET) /usr/local/bin/

# Help target
help:
	@echo "Available targets:"
	@echo "  all     - Build the project"
	@echo "  test    - Run tests"
	@echo "  clean   - Remove build artifacts"
	@echo "  install - Install to system"