package configfilevalidator

import "fmt"

// Version information set by link flags during build. We fall back to these sane
// default values when not provided
var (
	version = "unknown"
)

// Version contains config-file-validator version information
type Version struct {
	Version string
}

// String outputs the version as a string
func (v Version) String() string {
	return fmt.Sprintf("validator version %v", v.Version)
}

// GetVersion returns the version information
func GetVersion() Version {
	return Version{
		Version: version,
	}
}
