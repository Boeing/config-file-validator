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
	Validator  validator.Validator
}

// Instance of the FileType object to
// represent a JSON file
var JSONFileType = FileType{
	"json",
	tools.ArrToMap("json"),
	validator.JSONValidator{},
}

// Instance of the FileType object to
// represent a YAML file
var YAMLFileType = FileType{
	"yaml",
	tools.ArrToMap("yml", "yaml"),
	validator.YAMLValidator{},
}

// Instance of FileType object to
// represent a XML file
var XMLFileType = FileType{
	"xml",
	tools.ArrToMap("xml"),
	validator.XMLValidator{},
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	"toml",
	tools.ArrToMap("toml"),
	validator.TomlValidator{},
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	"ini",
	tools.ArrToMap("ini"),
	validator.IniValidator{},
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	"properties",
	tools.ArrToMap("properties"),
	validator.PropValidator{},
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	"hcl",
	tools.ArrToMap("hcl"),
	validator.HclValidator{},
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	"plist",
	tools.ArrToMap("plist"),
	validator.PlistValidator{},
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	"csv",
	tools.ArrToMap("csv"),
	validator.CsvValidator{},
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	"hocon",
	tools.ArrToMap("hocon"),
	validator.HoconValidator{},
}

// Instance of the FileType object to
// represent a ENV file
var EnvFileType = FileType{
	"env",
	tools.ArrToMap("env"),
	validator.EnvValidator{},
}

// Instance of the FileType object to
// represent an EDITORCONFIG file
var EditorConfigFileType = FileType{
	"editorconfig",
	tools.ArrToMap("editorconfig"),
	validator.EditorConfigValidator{},
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
	EditorConfigFileType,
}
