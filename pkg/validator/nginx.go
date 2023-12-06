package validator

import (
	"fmt"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

type NginxValidator struct{}

func (nv NginxValidator) Validate(b []byte) (result bool, errMsg error) {
	result = true

	defer func() {
		if r := recover(); r != nil {
			result = false
			errMsg = fmt.Errorf("%v", r)
		}
	}()

	p := parser.NewStringParser(string(b))
	_ = p.Parse()

	return result, errMsg

}
