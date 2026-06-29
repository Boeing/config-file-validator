## TOML validator silently passes files with duplicate keys

The TOML validator reports files with duplicate keys or duplicate table headers as valid. The TOML spec forbids both.

### Cause

`ValidateSyntax` in `pkg/validator/toml.go` only handles `*toml.DecodeError`:

```go
err := toml.Unmarshal(b, &output)
var derr *toml.DecodeError
if errors.As(err, &derr) {
    return false, &ValidationError{...}
}
return true, nil  // ← returns true even when err != nil
```

`go-toml/v2` returns a plain `*errors.errorString` for duplicate keys/tables, not a `*toml.DecodeError`. The error is silently swallowed.

### Fix

Add a fallback after the `DecodeError` check:

```go
if err != nil {
    return false, err
}
return true, nil
```

This won't include line/column info for duplicate key errors, but the file will correctly fail validation.

### Upstream

`go-toml/v2` has a known issue for this: [pelletier/go-toml#668](https://github.com/pelletier/go-toml/issues/668). A PR to wrap these as `DecodeError` with position info was submitted: [pelletier/go-toml#1065](https://github.com/pelletier/go-toml/pull/1065). Once that merges and we bump the dependency, our existing `errors.As` check will handle duplicate keys with full line/column reporting automatically.

In the meantime, the fallback fix ensures duplicate keys fail validation (just without position info).
