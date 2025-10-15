# Invalid justfile with syntax errors
set shell := ["bash", "-c"]

# Missing colon after recipe name
default:
    @echo "Hello, World!"

# Invalid recipe name starting with number
123invalid:
    @echo "This should fail"

# Invalid variable name
123invalid-var := "invalid"

# Invalid set directive
set invalid-directive = "value"

# Recipe with invalid parameter name
test 123param:
    @echo "This should fail"

# Invalid import statement
import "path with spaces"

# Recipe with invalid indentation (2 spaces instead of 4)
test-indent:
  @echo "This should fail - wrong indentation"

# Empty recipe name
:
    @echo "This should fail"