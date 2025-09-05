package validator

import (
	"bufio"
	"bytes"
	"errors"
	"regexp"
)

type MakefileValidator struct{}

var makefileTarget = regexp.MustCompile(`^[A-Za-z0-9_.-]+:`)
var maybeTarget = regexp.MustCompile(`^[A-Za-z0-9_.-]+\s+[A-Za-z0-9_.-]+.*$`)
var standaloneTarget = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func (MakefileValidator) Validate(b []byte) (bool, error) {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	inRule := false

	for scanner.Scan() {
		line := scanner.Text()

		// skip blank lines and comments
		if line == "" || line[0] == '#' {
			continue
		}

		// valid target
		if makefileTarget.MatchString(line) {
			inRule = true
			continue
		}

		// target-like line but missing colon
		if maybeTarget.MatchString(line) {
			return false, errors.New("invalid target line: missing colon")
		}

		// standalone target name without colon
		if standaloneTarget.MatchString(line) {
			return false, errors.New("invalid target line: missing colon")
		}

		// inside a rule, commands must start with TAB
		if inRule && len(line) > 0 && line[0] != '\t' && !makefileTarget.MatchString(line) {
			return false, errors.New("command does not start with TAB")
		}
	}

	return true, nil
}
