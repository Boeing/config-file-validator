// Package gojust parses and validates justfiles.
//
// Use [Parse] to parse justfile content from bytes, or [ParseFile] to
// parse from disk with automatic import and module resolution.
// Call [Justfile.Validate] to run semantic analysis and get diagnostics.
package gojust
