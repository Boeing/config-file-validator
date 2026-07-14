# goccy/go-yaml: Trailing newline changes parse result for explicit key with `>` value

**Describe the bug**

`yaml.Unmarshal` produces different results for `? >0` depending on whether a trailing newline is present. Without a newline it parses successfully as a mapping with key `>0`. With a newline it fails, interpreting `>0` as a block scalar header.

**To Reproduce**

```go
package main

import (
	"fmt"
	goyaml "github.com/goccy/go-yaml"
)

func main() {
	var v1, v2 any

	// Without trailing newline — succeeds
	err1 := goyaml.Unmarshal([]byte("? >0"), &v1)
	fmt.Printf("without newline: %v (err: %v)\n", v1, err1)

	// With trailing newline — fails
	err2 := goyaml.Unmarshal([]byte("? >0\n"), &v2)
	fmt.Printf("with newline:    %v (err: %v)\n", v2, err2)
}
```

Output:
```
without newline: map[>0:<nil>] (err: <nil>)
with newline:    map[:<nil>] (err: [1:3] invalid header option: "0"
>  1 | ? >0
         ^
)
```

**Expected behavior**

Both inputs should parse identically. `>0` here follows the `?` explicit key indicator and should be treated as a plain scalar value, not a block scalar header. A trailing newline should not change how the token is classified.

**Version Variables**
- Go version: 1.26.3
- go-yaml's Version: v1.19.2

**Additional context**

We encountered this while building a YAML formatting tool that appends a final newline to files (standard practice). Files containing explicit key indicators with values that resemble block scalar headers become unparseable after the newline is added.
