package validator

import "testing"

func TestMakefileValidator_Validate(t *testing.T) {
	v := MakefileValidator{}

	t.Run("valid makefile - single target", func(t *testing.T) {
		content := []byte(`
build: main.go
	go build -o app main.go
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile, got error: %v", err)
		}
	})

	t.Run("valid makefile - multiple targets", func(t *testing.T) {
		content := []byte(`
build: main.go
	go build -o app main.go

test: build
	go test ./...
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile with multiple targets, got error: %v", err)
		}
	})

	t.Run("valid makefile - with comments and blank lines", func(t *testing.T) {
		content := []byte(`
# this is a comment

build: main.go
	go build -o app main.go
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile with comments, got error: %v", err)
		}
	})

	t.Run("valid makefile - target without commands", func(t *testing.T) {
		content := []byte(`
clean:
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile with empty target, got error: %v", err)
		}
	})

	t.Run("valid makefile - multiple commands in one target", func(t *testing.T) {
		content := []byte(`
build:
	go build -o app main.go
	echo "done"
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile with multiple commands, got error: %v", err)
		}
	})

	t.Run("invalid makefile - command without TAB", func(t *testing.T) {
		content := []byte(`
build: main.go
go build -o app main.go
`)
		ok, err := v.Validate(content)
		if ok || err == nil {
			t.Errorf("expected invalid makefile (command without TAB), but got valid")
		}
	})

	t.Run("invalid makefile - spaces instead of TAB", func(t *testing.T) {
		content := []byte(`
build:
    go build -o app main.go
`)
		ok, err := v.Validate(content)
		if ok || err == nil {
			t.Errorf("expected invalid makefile (spaces instead of TAB), but got valid")
		}
	})

	t.Run("invalid makefile - target without colon", func(t *testing.T) {
		content := []byte(`
build
	go build -o app main.go
`)
		ok, err := v.Validate(content)
		if ok || err == nil {
			t.Errorf("expected invalid makefile (missing colon in target), but got valid")
		}
	})

	t.Run("invalid makefile - target with dependencies but missing colon", func(t *testing.T) {
		content := []byte(`
build main.go
	go build -o app main.go
`)
		ok, err := v.Validate(content)
		if ok || err == nil {
			t.Errorf("expected invalid makefile (target with deps missing colon), but got valid")
		}
	})

	t.Run("valid makefile - empty file", func(t *testing.T) {
		content := []byte(``)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid for empty file, got error: %v", err)
		}
	})

	t.Run("valid makefile - only comments", func(t *testing.T) {
		content := []byte(`
# this is a comment
# another comment
`)
		ok, err := v.Validate(content)
		if !ok || err != nil {
			t.Errorf("expected valid makefile with only comments, got error: %v", err)
		}
	})
}
