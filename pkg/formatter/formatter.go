package formatter

// formatter is the interface that wraps the basic Format method

// Format accepts a byte array of a file or string to be formatted
// and returns the formatted byte array.
type Formatter interface {
	Format(f []byte) ([]byte, error)
}
