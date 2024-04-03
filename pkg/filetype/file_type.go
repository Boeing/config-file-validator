package filetype

import (
	"github.com/Boeing/config-file-validator/pkg/validator"
)

// The FileType object stores information
// about a file type including name, extensions,
// as well as an instance of the file type's validator
// to be able to validate the file
type FileType struct {
	Name       string
	Extensions []string
	Validator  validator.Validator
}

// Instance of the FileType object to
// represent a JSON file
var JsonFileType = FileType{
	"json",
	[]string{"json"},
	validator.JsonValidator{},
}

// Instance of the FileType object to
// represent a YAML file
var YamlFileType = FileType{
	"yaml",
	[]string{"yml", "yaml"},
	validator.YamlValidator{},
}

// Instance of FileType object to
// represent a XML file
var XmlFileType = FileType{
	"xml",
	[]string{"xml"},
	validator.XmlValidator{},
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	"toml",
	[]string{"toml"},
	validator.TomlValidator{},
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	"ini",
	[]string{"ini"},
	validator.IniValidator{},
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	"properties",
	[]string{"properties"},
	validator.PropValidator{},
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	"hcl",
	[]string{"hcl"},
	validator.HclValidator{},
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	"plist",
	[]string{"plist"},
	validator.PlistValidator{},
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	"csv",
	[]string{"csv"},
	validator.CsvValidator{},
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	"hocon",
	[]string{"hocon"},
	validator.HoconValidator{},
}

// An array of files types that are supported
// by the validator
var FileTypes = []FileType{
	JsonFileType,
	YamlFileType,
	XmlFileType,
	TomlFileType,
	IniFileType,
	PropFileType,
	HclFileType,
	PlistFileType,
	CsvFileType,
	HoconFileType,
}
