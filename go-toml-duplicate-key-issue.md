## Duplicate key/table errors return plain errorString instead of *DecodeError

When `Unmarshal` encounters a duplicate key or duplicate table, it returns a `*errors.errorString` instead of a `*toml.DecodeError`. This means consumers can't extract position information from these errors.

### Reproduction

```go
var out any
err := toml.Unmarshal([]byte("key = 1\nkey = 2\n"), &out)
fmt.Printf("type: %T\n", err)  // *errors.errorString
fmt.Printf("err: %v\n", err)   // toml: key key is already defined

var derr *toml.DecodeError
fmt.Println(errors.As(err, &derr))  // false
```

### Expected

`err` should be a `*toml.DecodeError` with line/column pointing to the second `key` definition, consistent with how other parse errors are reported.

### Impact

Consumers that use `errors.As(err, &derr)` to extract position info (for editor annotations, CI output, etc.) silently miss duplicate key errors because the type assertion fails. The only workaround is string-parsing the error message and scanning the source manually.

### Version

go-toml v2.3.1
