package filetype

import (
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

// registerFormatters sets the Formatter field on each FileType in the
// FileTypes slice. This must run after the main init() that builds FileTypes.
//
// We update the slice entries directly because FileTypes holds value copies —
// updating the package-level vars (JSONFileType etc.) has no effect on the
// already-copied slice.
func init() {
	for i, ft := range FileTypes {
		switch ft.Name {
		case "json":
			FileTypes[i].Formatter = jsonfmt.Formatter{}
		case "yaml":
			FileTypes[i].Formatter = yamlfmt.Formatter{}
		case "hcl":
			FileTypes[i].Formatter = hclfmt.Formatter{}
		default:
			// no formatter registered for this type yet
		}
	}
}
