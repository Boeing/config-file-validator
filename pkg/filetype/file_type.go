//go:generate go run ../../internal/generate/knownfiles/main.go

// Package filetype defines the supported file types and their validators.
//
// KnownFiles are populated at init time from three sources:
//  1. LinguistKnownFiles (generated from GitHub Linguist's languages.yml)
//  2. extraKnownFiles (manual entries not in Linguist, e.g. .shellcheckrc)
//  3. excludeKnownFiles (auto-detected conflicts with dedicated validators)
//
// Filenames in excludeKnownFiles are skipped during population. A conflict
// is detected when a Linguist filename has an extension that belongs to a
// file type outside fileTypeRegistry (e.g. .editorconfig → EditorConfig).
//go:generate go run ../../../internal/generate/knownfiles
//go:generate go run ../../internal/generate/knownfiles

package filetype

import (
	"strings"

	"github.com/Boeing/config-file-validator/v2/pkg/validator"
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
	Extensions: arrToMap("json"),
	Validator:  validator.JSONValidator{},
}

// Instance of the FileType object to
// represent a YAML file
var YAMLFileType = FileType{
	Name:       "yaml",
	Extensions: arrToMap("yml", "yaml"),
	Validator:  validator.YAMLValidator{},
}

// Instance of FileType object to
// represent a XML file
var XMLFileType = FileType{
	Name:       "xml",
	Extensions: arrToMap("xml"),
	Validator:  validator.XMLValidator{},
}

// Instance of FileType object to
// represent a Toml file
var TomlFileType = FileType{
	Name:       "toml",
	Extensions: arrToMap("toml"),
	Validator:  validator.TomlValidator{},
}

// Instance of FileType object to
// represent a Ini file
var IniFileType = FileType{
	Name:       "ini",
	Extensions: arrToMap("ini"),
	Validator:  validator.IniValidator{},
}

// Instance of FileType object to
// represent a Properties file
var PropFileType = FileType{
	Name:       "properties",
	Extensions: arrToMap("properties"),
	Validator:  validator.PropValidator{},
}

// Instance of the FileType object to
// represent a HCL file
var HclFileType = FileType{
	Name:       "hcl",
	Extensions: arrToMap("hcl", "tf", "tfvars"),
	Validator:  validator.HclValidator{},
}

// Instance of the FileType object to
// represent a Plist file
var PlistFileType = FileType{
	Name:       "plist",
	Extensions: arrToMap("plist"),
	Validator:  validator.PlistValidator{},
}

// Instance of the FileType object to
// represent a CSV file
var CsvFileType = FileType{
	Name:       "csv",
	Extensions: arrToMap("csv"),
	Validator:  validator.CsvValidator{},
}

// Instance of the FileType object to
// represent a HOCON file
var HoconFileType = FileType{
	Name:       "hocon",
	Extensions: arrToMap("hocon"),
	Validator:  validator.HoconValidator{},
}

// Instance of the FileType object to
// represent a ENV file
var EnvFileType = FileType{
	Name:       "env",
	Extensions: arrToMap("env"),
	Validator:  validator.EnvValidator{},
}

// Instance of the FileType object to
// represent an EDITORCONFIG file
var EditorConfigFileType = FileType{
	Name:       "editorconfig",
	Extensions: arrToMap("editorconfig"),
	Validator:  validator.EditorConfigValidator{},
}

// Instance of the FileType object to
// represent a TOON file
var ToonFileType = FileType{
	Name:       "toon",
	Extensions: arrToMap("toon"),
	Validator:  validator.ToonValidator{},
}

// Instance of the FileType object to
// represent a Sarif file
var SarifFileType = FileType{
	Name:       "sarif",
	Extensions: arrToMap("sarif"),
	Validator:  validator.SarifValidator{},
}

var JSONCFileType = FileType{
	Name:       "jsonc",
	Extensions: arrToMap("jsonc"),
	Validator:  validator.JSONCValidator{},
}

var JustfileFileType = FileType{
	Name:       "justfile",
	Extensions: arrToMap("just"),
	KnownFiles: map[string]struct{}{
		"justfile":  {},
		"Justfile":  {},
		".justfile": {},
	},
	Validator: validator.JustfileValidator{},
}

// extraKnownFiles contains manual entries not covered by Linguist.
var extraKnownFiles = map[string][]string{
	"ini": {
		".shellcheckrc",
	},
	"justfile": {
		"justfile",
		"Justfile",
		".justfile",
	},
}

// fileTypeRegistry maps file type names to their package-level variables.
var fileTypeRegistry = map[string]*FileType{
	"json":       &JSONFileType,
	"jsonc":      &JSONCFileType,
	"yaml":       &YAMLFileType,
	"xml":        &XMLFileType,
	"toml":       &TomlFileType,
	"ini":        &IniFileType,
	"properties": &PropFileType,
	"hcl":        &HclFileType,
	"plist":      &PlistFileType,
	"csv":        &CsvFileType,
	"hocon":      &HoconFileType,
	"env":        &EnvFileType,
	"toon":       &ToonFileType,
	"sarif":      &SarifFileType,
	"justfile":   &JustfileFileType,
}

// excludeKnownFiles lists Linguist entries to skip because we have
// a dedicated file type for them (e.g. .editorconfig → EditorConfig, not INI).
// Built automatically in init() by detecting filenames whose extension
// matches a file type that exists outside the Linguist registry.
var excludeKnownFiles map[string]struct{}

func init() {
	// Collect extensions from types NOT in fileTypeRegistry.
	// These have dedicated validators that Linguist doesn't know about.
	registeredExts := make(map[string]struct{})
	for _, ft := range fileTypeRegistry {
		for ext := range ft.Extensions {
			registeredExts[ext] = struct{}{}
		}
	}
	unregisteredExts := make(map[string]struct{})
	for _, ft := range []FileType{EditorConfigFileType} {
		for ext := range ft.Extensions {
			if _, inRegistry := registeredExts[ext]; !inRegistry {
				unregisteredExts[ext] = struct{}{}
			}
		}
	}

	// Any Linguist filename whose extension matches an unregistered type
	// should be excluded — it has its own dedicated validator.
	excludeKnownFiles = make(map[string]struct{})
	for name := range fileTypeRegistry {
		for _, f := range LinguistKnownFiles[name] {
			if ext := extOf(f); ext != "" {
				if _, unregistered := unregisteredExts[ext]; unregistered {
					excludeKnownFiles[f] = struct{}{}
				}
			}
		}
	}

	for name, ft := range fileTypeRegistry {
		ft.KnownFiles = make(map[string]struct{})
		for _, f := range LinguistKnownFiles[name] {
			if _, skip := excludeKnownFiles[f]; skip {
				continue
			}
			ft.KnownFiles[f] = struct{}{}
		}
		for _, f := range extraKnownFiles[name] {
			ft.KnownFiles[f] = struct{}{}
		}
	}

	// Build FileTypes after KnownFiles are populated so the slice
	// contains the fully initialized values.
	FileTypes = []FileType{
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
		ToonFileType,
		SarifFileType,
		JSONCFileType,
		JustfileFileType,
	}
}

// FileTypes contains all file types supported by the validator.
// Populated in init() after KnownFiles are merged from Linguist data.
var FileTypes []FileType

func arrToMap(args ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(args))
	for _, item := range args {
		m[item] = struct{}{}
	}
	return m
}

// extOf returns the extension of a filename (lowercase, without the dot),
// matching filepath.Ext behavior. For ".editorconfig" returns "editorconfig".
// For files with no dot returns "".
func extOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return strings.ToLower(name[i+1:])
		}
	}
	return ""
}
