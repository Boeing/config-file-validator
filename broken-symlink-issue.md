## Broken symlink aborts entire validation run

A single broken symlink in a scanned directory causes the validator to abort with exit 2, skipping all remaining files.

### Reproduction

Create a broken symlink in a directory with valid config files:

```
mkdir project && cd project
echo '{}' > valid.json
echo 'key: value' > valid.yaml
ln -sf /nonexistent broken.json

validator .
```

A symlink is "broken" when its target doesn't exist. Common ways this happens:
- `ln -sf /nonexistent broken.json` (target never existed)
- Create a symlink to a real file, then delete the target

Output:

```
An error occurred during CLI execution: unable to read file: open /path/to/broken.json: no such file or directory
```

`valid.json` and `valid.yaml` are never checked.

### Testing

The testscript framework supports symlinks. Use the `[symlink]` condition to skip on platforms without symlink support:

```
[symlink]

# Broken symlink should fail but not abort the run
symlink broken.json -> nonexistent.json

exec validator .
stdout '×'
stdout 'broken symlink'
stdout '✓'

-- good.json --
{"valid": true}
```

For unit tests in `pkg/finder`, create a broken symlink with `os.Symlink`:

```go
dir := t.TempDir()
testhelper.WriteFile(t, dir, "good.json", `{}`)
os.Symlink("/nonexistent", filepath.Join(dir, "broken.json"))
```

### Expected behavior

Report the broken symlink as a failed file and continue validating the rest:

```
    × broken.json
        error: broken symlink: target does not exist
    ✓ valid.json
    ✓ valid.yaml

Summary: 2 succeeded, 1 failed
```

Exit 1 because there's a failure, but the other files still get checked.

### Impact

Broken symlinks are common in real repos — stale after branch switches, submodule changes, or partial clones. One stale symlink shouldn't prevent the rest of the project from being validated, especially in CI where the exit code gates the pipeline.

### Suggested fix

In the file walker, when reading a file returns `os.ErrNotExist` (broken symlink), add a failed report for that file with a clear error message and continue walking. Don't return the error up the stack.
