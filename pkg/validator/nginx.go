package validator

import (
	"github.com/tufanbarisyildirim/gonginx/parser"
)

type NginxValidator struct{}

func (nv NginxValidator) Validate(b []byte) (result bool, errMsg error) {
	p := parser.NewStringParser(string(b))
	_, err := p.Parse()
	if err != nil {
		return false, err
	}

	return true, nil
}
