package filetype

import (
	"github.com/Boeing/config-file-validator/pkg/tools"
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
	Extensions: tools.ArrToMap("json"),
	Validator:  validator.JSONValidator{},
}

// Instance of the FileType object to
// represent a YAML file
var YAMLFileType = FileType{
	Name:       "yaml",
	Extensions: tools.ArrToMap("yml", "yaml"),
	KnownFiles: tools.ArrToMap(
		".clang-format",
		".clang-tidy",
		".clangd",
		".gemrc",
	),
	Validator: validator.YAMLValidator{},
}

// Instance of FileType object to
// represent a XML file
var XMLFileType = FileType{
	Name:       "xml",
	Extensions: tools.ArrToMap("xml"),
	Validator:  validator.XMLValidator{},
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	Name:       "toml",
	Extensions: tools.ArrToMap("toml"),
	Validator:  validator.TomlValidator{},
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	Name:       "ini",
	Extensions: tools.ArrToMap("ini"),
	KnownFiles: tools.ArrToMap(
		".editorconfig",
		".gitconfig",
		".gitmodules",
		".shellcheckrc",
		".npmrc",
		"inputrc",
		".inputrc",
		".wgetrc",
		".curlrc",
		".nanorc",
		".flake8",
		".pylintrc",
	),
	Validator: validator.IniValidator{},
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	Name:       "properties",
	Extensions: tools.ArrToMap("properties"),
	Validator:  validator.PropValidator{},
}

// Instance of the FileType object to
// represent a Pkl file
var PklFileType = FileType{
	"pkl",
	tools.ArrToMap("pkl"),
	validator.PklValidator{},
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	Name:       "hcl",
	Extensions: tools.ArrToMap("hcl"),
	Validator:  validator.HclValidator{},
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	Name:       "plist",
	Extensions: tools.ArrToMap("plist"),
	Validator:  validator.PlistValidator{},
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	Name:       "csv",
	Extensions: tools.ArrToMap("csv"),
	Validator:  validator.CsvValidator{},
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	Name:       "hocon",
	Extensions: tools.ArrToMap("hocon"),
	Validator:  validator.HoconValidator{},
}

// Instance of the FileType object to
// represent a ENV file
var EnvFileType = FileType{
	Name:       "env",
	Extensions: tools.ArrToMap("env"),
	Validator:  validator.EnvValidator{},
}

// Instance of the FileType object to
// represent an EDITORCONFIG file
var EditorConfigFileType = FileType{
	Name:       "editorconfig",
	Extensions: tools.ArrToMap("editorconfig"),
	Validator:  validator.EditorConfigValidator{},
}

// Instance of the FileType object to
// represent a TOON file
var ToonFileType = FileType{
	Name:       "toon",
	Extensions: tools.ArrToMap("toon"),
	Validator:  validator.ToonValidator{},
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
	PklFileType,
	HclFileType,
	PlistFileType,
	CsvFileType,
	HoconFileType,
	EnvFileType,
	EditorConfigFileType,
	ToonFileType,
}
