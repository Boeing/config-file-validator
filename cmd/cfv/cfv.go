/*
cfv validates, formats, and fixes configuration files across 18 formats.

Usage: cfv [global-flags] [subcommand] [subcommand-flags] [<search_path>...]

Subcommands:

	check    Validate syntax and schema (default when no subcommand given)
	format   Report formatting issues (use --fix to rewrite files)
	version  Print version and exit
	help     Print help; "cfv help <subcommand>" for subcommand help

positional arguments:

	search_path: Filesystem path to search for configuration files.
	             Defaults to the current working directory.
	             Multiple paths can be provided separated by spaces.
	             Use "-" to read from stdin (requires --file-types).

Schema validation runs automatically when a file declares a schema:

	YAML:  # yaml-language-server: $schema=schema.json
	XML:   xsi:noNamespaceSchemaLocation="schema.xsd"
	XML:   <!DOCTYPE> with inline DTD (validated during syntax check)

For JSON, JSONC, TOML, and TOON, use --schema-map or --schemastore.

Global flags apply to all subcommands and must precede the subcommand name.
Run "cfv help check" or "cfv help format" for subcommand-specific flags.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	configfilevalidator "github.com/Boeing/config-file-validator/v3"
	"github.com/Boeing/config-file-validator/v3/pkg/cli"
	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
)

// =============================================================================
// Subcommand router
// =============================================================================

// mainInit is the testable entry point. Returns an exit code.
func mainInit() int {
	args := os.Args[1:]

	// Phase 1: parse global flags. Only --version and --help live here.
	// All other flags belong to the subcommand FlagSet.
	globalFS := flag.NewFlagSet("cfv", flag.ContinueOnError)
	globalFS.Usage = printUsage
	versionFlag := globalFS.Bool("version", false, "Print the version and exit.")
	// Suppress the default error output — we handle it below.
	globalFS.SetOutput(io.Discard)

	// Parse only until the first non-flag argument (the subcommand or a path).
	// flag.ContinueOnError means unknown flags return an error rather than exiting,
	// which lets us forward unrecognised flags to the subcommand FlagSet.
	globalErr := globalFS.Parse(args)
	if errors.Is(globalErr, flag.ErrHelp) {
		return 0
	}
	remaining := globalFS.Args()

	if *versionFlag {
		fmt.Println(configfilevalidator.GetVersion())
		return 0
	}

	// No arguments at all: run check on current directory.
	if len(args) == 0 {
		return runCheck(args)
	}

	// Phase 2: detect subcommand from the first non-flag token.
	// If global flag parsing consumed everything, remaining is empty —
	// treat that as a bare check too.
	subArgs := remaining
	if len(remaining) > 0 {
		switch remaining[0] {
		case "check":
			return runCheck(remaining[1:])
		case "format":
			return runFormat(remaining[1:])
		case "version":
			fmt.Println(configfilevalidator.GetVersion())
			return 0
		case "help":
			if len(remaining) > 1 {
				switch remaining[1] {
				case "check":
					printCheckUsage()
					return 0
				case "format":
					printFormatUsage()
					return 0
				}
			}
			printUsage()
			return 0
		}
		// Not a known subcommand — treat the full original args as a bare
		// check invocation so flags like --reporter still work.
		subArgs = args
	}

	// Bare invocation: cfv [flags] [paths] with no subcommand keyword.
	return runCheck(subArgs)
}

func main() {
	os.Exit(mainInit())
}

// =============================================================================
// Usage
// =============================================================================

func printUsage() {
	fmt.Println("Usage: cfv [global-flags] <subcommand> [subcommand-flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  check    Validate syntax and schema (equivalent to v2 'validator')")
	fmt.Println("  format   Report formatting issues; use --fix to rewrite files [Phase 2]")
	fmt.Println("  version  Print version and exit")
	fmt.Println("  help     Print this help; 'cfv help <subcommand>' for details")
	fmt.Println()
	fmt.Println("Running 'cfv [flags] [paths]' without a subcommand runs check.")
	fmt.Println()
	fmt.Println("Run 'cfv help check' for the full flag reference.")
}

func printCheckUsage() {
	fmt.Println("Usage: cfv check [flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Validate configuration files for syntax and schema errors.")
	fmt.Println("Equivalent to the v2 'validator' command.")
	fmt.Println()
	fmt.Println("positional arguments:")
	fmt.Println("  search_path  Path to search. Defaults to '.'. Use '-' for stdin.")
	fmt.Println()
	fmt.Println("Schema validation runs automatically when a file declares a schema:")
	fmt.Println("  YAML:  # yaml-language-server: $schema=schema.json")
	fmt.Println("  XML:   xsi:noNamespaceSchemaLocation=\"schema.xsd\"")
	fmt.Println("  XML:   <!DOCTYPE> with inline DTD (validated during syntax check)")
	fmt.Println()
	fmt.Println("For JSON, JSONC, TOML, and TOON, use --schema-map or --schemastore.")
	fmt.Println()
	fmt.Println("flags:")
	// Flag defaults are printed by the subcommand's FlagSet after parsing.
	// When called from --help during flag parsing, the FlagSet prints defaults
	// to its own output automatically; this branch handles the cfv help check case.
}

func printFormatUsage() {
	fmt.Println("Usage: cfv format [--fix] [flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Report formatting issues. Use --fix to rewrite files.")
	fmt.Println()
	var fmtNames []string
	for _, ft := range filetype.FileTypes {
		if ft.Formatter != nil {
			fmtNames = append(fmtNames, ft.Name)
		}
	}
	fmt.Printf("Formats with registered formatters: %s\n", strings.Join(fmtNames, ", "))
	fmt.Println("Run 'cfv format --help' for the full flag reference.")
}

// =============================================================================
// check subcommand
// =============================================================================

// runCheck implements "cfv check [flags] [paths]" and the bare "cfv [flags] [paths]".
// Behavior is identical to the v2 validator binary.
func runCheck(args []string) int {
	cfg, err := parseCheckFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err.Error())
		printCheckUsage()
		return 2
	}

	resolved, err := resolveCheckConfig(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
		return 2
	}

	if resolved.watch {
		exitStatus, err := runWatch(resolved)
		if err != nil {
			log.Printf("An error occurred during watch execution: %v", err)
		}
		return exitStatus
	}

	optsResolver, prettierCfg := buildFormatOptionsResolver(&cfg, resolved)
	c := buildCLI(resolved,
		cli.WithFormatOptions(optsResolver),
		cli.WithFormatIgnores(buildFormatIgnores(&cfg, resolved)),
	)
	exitStatus, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
	}
	printPrettierWarnings(prettierCfg)
	return exitStatus
}

// =============================================================================
// format subcommand
// =============================================================================

// runFormat implements "cfv format [flags] [paths]".
// Reports formatting issues. With --fix, rewrites files to canonical style.
func runFormat(args []string) int {
	cfg, err := parseFormatFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err.Error())
		printFormatUsage()
		return 2
	}

	resolved, err := resolveFormatConfig(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
		return 2
	}

	c := buildCLI(resolved, cli.WithFormatIgnores(buildFormatIgnores(&cfg, resolved)))

	// Build the per-format options resolver. This implements the cascade:
	// CLI flags > [format.<type>] > [format] > format-specific defaults.
	optsResolver, prettierCfg := buildFormatOptionsResolver(&cfg, resolved)

	exitStatus, err := c.Format(optsResolver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
	}
	printPrettierWarnings(prettierCfg)
	return exitStatus
}
