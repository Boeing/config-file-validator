package filetype

import (
	"github.com/Boeing/config-file-validator/pkg/formatter"
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
	Formatter  formatter.Formatter
}

// Instance of the FileType object to
// represent a JSON file
var JSONFileType = FileType{
	"json",
	tools.ArrToMap("json"),
	validator.JSONValidator{},
	formatter.JSONFormatter{},
}

// Instance of the FileType object to
// represent a YAML file
var YAMLFileType = FileType{
	"yaml",
	tools.ArrToMap("yml", "yaml"),
	validator.YAMLValidator{},
	nil,
}

// Instance of FileType object to
// represent a XML file
var XMLFileType = FileType{
	"xml",
	tools.ArrToMap("xml"),
	validator.XMLValidator{},
	nil,
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	"toml",
	tools.ArrToMap("toml"),
	validator.TomlValidator{},
	nil,
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	"ini",
	tools.ArrToMap("ini"),
	validator.IniValidator{},
	nil,
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	"properties",
	tools.ArrToMap("properties"),
	validator.PropValidator{},
	nil,
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	"hcl",
	tools.ArrToMap("hcl"),
	validator.HclValidator{},
	nil,
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	"plist",
	tools.ArrToMap("plist"),
	validator.PlistValidator{},
	nil,
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	"csv",
	tools.ArrToMap("csv"),
	validator.CsvValidator{},
	nil,
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	"hocon",
	tools.ArrToMap("hocon"),
	validator.HoconValidator{},
	nil,
}

// Instance of the FileType object to
// represent a ENV file
var EnvFileType = FileType{
	"env",
	tools.ArrToMap("env"),
	validator.EnvValidator{},
	nil,
}

// Instance of the FileType object to
// represent an EDITORCONFIG file
var EditorConfigFileType = FileType{
	"editorconfig",
	tools.ArrToMap("editorconfig"),
	validator.EditorConfigValidator{},
	nil,
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
