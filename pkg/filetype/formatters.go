package filetype

import (
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/envfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/inifmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/propfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/tomlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

// init registers formatters with their corresponding FileTypes. This must run
// after the main init() in file_type.go that builds the FileTypes slice (Go
// processes init() functions in filename-sorted order within a package).
//
// We update the slice entries directly because FileTypes holds value copies —
// updating the package-level vars (JSONFileType etc.) has no effect on the
// already-copied slice.
func init() {
	for i, ft := range FileTypes {
		switch ft.Name {
		case "json":
			FileTypes[i].Formatter = jsonfmt.Formatter{}
		case "jsonc":
			FileTypes[i].Formatter = jsoncfmt.Formatter{}
		case "yaml":
			FileTypes[i].Formatter = yamlfmt.Formatter{}
		case "hcl":
			FileTypes[i].Formatter = hclfmt.Formatter{}
		case "xml":
			FileTypes[i].Formatter = xmlfmt.Formatter{}
		case "toml":
			FileTypes[i].Formatter = tomlfmt.Formatter{}
		case "ini":
			FileTypes[i].Formatter = inifmt.Formatter{}
		case "env":
			FileTypes[i].Formatter = envfmt.Formatter{}
		case "properties":
			FileTypes[i].Formatter = propfmt.Formatter{}
		default:
			// no formatter registered for this type yet
		}
	}
}
