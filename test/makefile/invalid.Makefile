# Invalid Makefile with syntax errors
.PHONY: all clean test

# Missing colon after target
all build test:
	@echo "Building project..."
	gcc -o main main.c

# Invalid target name with spaces
invalid target:
	@echo "This should fail"

# Recipe with spaces instead of tab
test:
    @echo "This should fail - spaces instead of tab"

# Empty target
empty:
	@echo "This is fine"

# Target with invalid characters
invalid@target:
	@echo "This should fail"