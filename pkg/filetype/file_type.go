package filetype

import (
	"github.com/Boeing/config-file-validator/pkg/misc"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

// The FileType object stores information
// about a file type including name, extensions,
// as well as an instance of the file type's validator
// to be able to validate the file
type FileType struct {
	Name       string
	Extensions map[string]struct{}
	KnownFiles map[string]struct{}
	Validator  validator.Validator
}

// Instance of the FileType object to
// represent a JSON file
var JSONFileType = FileType{
	Name:       "json",
	Extensions: misc.ArrToMap("json"),
	Validator:  validator.JSONValidator{},
}

// Instance of the FileType object to
// represent a YAML file
var YAMLFileType = FileType{
	Name:       "yaml",
	Extensions: misc.ArrToMap("yml", "yaml"),
	Validator:  validator.YAMLValidator{},
}

// Instance of FileType object to
// represent a XML file
var XMLFileType = FileType{
	Name:       "xml",
	Extensions: misc.ArrToMap("xml"),
	Validator:  validator.XMLValidator{},
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	Name:       "toml",
	Extensions: misc.ArrToMap("toml"),
	Validator:  validator.TomlValidator{},
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	Name:       "ini",
	Extensions: misc.ArrToMap("ini"),
	KnownFiles: misc.ArrToMap(".editorconfig", ".gitconfig", ".gitmodules", ".shellcheckrc", ".npmrc", "inputrc", ".inputrc", ".wgetrc", ".curlrc", ".nanorc"),
	Validator:  validator.IniValidator{},
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	Name:       "properties",
	Extensions: misc.ArrToMap("properties"),
	Validator:  validator.PropValidator{},
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	Name:       "hcl",
	Extensions: misc.ArrToMap("hcl", ".tftpl"),
	Validator:  validator.HclValidator{},
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	Name:       "plist",
	Extensions: misc.ArrToMap("plist"),
	Validator:  validator.PlistValidator{},
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	Name:       "csv",
	Extensions: misc.ArrToMap("csv"),
	Validator:  validator.CsvValidator{},
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	Name:       "hocon",
	Extensions: misc.ArrToMap("hocon"),
	Validator:  validator.HoconValidator{},
}

// Instance of the FileType object to
// represent a ENV file
var EnvFileType = FileType{
	Name:       "env",
	Extensions: misc.ArrToMap("env"),
	Validator:  validator.EnvValidator{},
}

// An array of files types that are supported
// by the validator
var FileTypes = []FileType{
	JSONFileType,
	YAMLFileType,
	XMLFileType,
	TomlFileType,
	IniFileType,
	PropFileType,
	HclFileType,
	PlistFileType,
	CsvFileType,
	HoconFileType,
	EnvFileType,
}
