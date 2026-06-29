## XML validator hangs on recursive entity expansion

The XML validator hangs indefinitely when parsing a file with recursive DTD entity definitions. The helium parser expands entities without depth or size limits, and we pass `context.Background()` with no timeout.

Any XML file with entities that reference other entities will cause unbounded expansion. The process pins a CPU core and never returns.

### Cause

`helium.NewParser().ValidateDTD(true).Parse(ctx, b)` has no expansion limit. Since we pass `context.Background()`, there's nothing to abort the parse.

### Suggested fix

Wrap the parse in a context with a timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_, err := helium.NewParser().ValidateDTD(true).Parse(ctx, b)
```

This preserves DTD validation for normal files while preventing the hang. If the timeout fires, return a validation error indicating the file took too long to parse.

### Impact

Low severity. Requires a deliberately crafted file. The only effect is the validator process hangs — no data leak or privilege escalation. Relevant for CI pipelines where a malicious file in a repo could stall the build.
