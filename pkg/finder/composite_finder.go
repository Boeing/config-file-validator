package finder

import "errors"

// CompositeFileFinder impliments composite pattern for FileFinder interface
type CompositeFileFinder struct {
	finders []FileFinder
}

func NewCompositeFileFinder(finders []FileFinder) *CompositeFileFinder {
	compFinder := &CompositeFileFinder{
		finders: finders,
	}
	return compFinder
}

// Find implements the FileFinder interface by recursively
// calling the Find method for FileFinders in the Composite
func (c *CompositeFileFinder) Find() ([]FileMetadata, error) {
	var matchingFiles []FileMetadata
	var errs error
	for _, finder := range c.finders {
		files, err := finder.Find()
		matchingFiles = append(matchingFiles, files...)
		errs = errors.Join(errs, err)
	}
	return matchingFiles, errs
}
