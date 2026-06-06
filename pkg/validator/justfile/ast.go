package gojust

// Justfile is the root AST node representing a parsed justfile.
type Justfile struct {
	Recipes     []*Recipe
	Assignments []*Assignment
	Unexports   []*Unexport
	Aliases     []*Alias
	Settings    []*Setting
	Imports     []*Import
	Modules     []*Module
	Comments    []*Comment
	File        string // source filename, empty for Parse()
}

type Comment struct {
	Value string
	Pos   Position
}

type Assignment struct {
	Name       string
	Value      Expression
	Export     bool
	Eager      bool
	Private    bool
	Parameters []string // non-empty for function definitions: name(params) := expr
	Pos        Position
}

type Alias struct {
	Name       string
	Target     string
	Attributes []*Attribute
	Pos        Position
}

type Unexport struct {
	Name string
	Pos  Position
}

type Setting struct {
	Name  string
	Value SettingValue
	Pos   Position
}

type SettingValue struct {
	Bool   *bool
	String *string
	List   []string
}

// Kind returns which type of value this setting holds: "bool", "string", "list", or "" if unset.
func (sv SettingValue) Kind() string {
	if sv.Bool != nil {
		return "bool"
	}
	if sv.String != nil {
		return "string"
	}
	if sv.List != nil {
		return "list"
	}
	return ""
}

// settingDefault describes a known justfile setting and its default value
// for JSON dump output. The Name uses kebab-case (justfile syntax);
// buildSettings converts to snake_case for JSON keys.
type settingDefault struct {
	Name    string
	Default any // false for bool settings, nil for string/list settings
}

// knownSettingsList is the single source of truth for all recognized settings.
// Both the analyzer (unknown-setting warnings) and the dump builder (JSON
// defaults) derive from this list.
var knownSettingsList = []settingDefault{
	{"allow-duplicate-recipes", false},
	{"allow-duplicate-variables", false},
	{"dotenv-filename", nil},
	{"dotenv-load", false},
	{"dotenv-override", false},
	{"dotenv-path", nil},
	{"dotenv-required", false},
	{"export", false},
	{"fallback", false},
	{"guards", false},
	{"ignore-comments", false},
	{"lazy", false},
	{"no-exit-message", false},
	{"positional-arguments", false},
	{"quiet", false},
	{"script-interpreter", nil},
	{"shell", nil},
	{"tempdir", nil},
	{"unstable", false},
	{"windows-powershell", false},
	{"windows-shell", nil},
	{"working-directory", nil},
}

type Import struct {
	Path     string
	Optional bool
	Justfile *Justfile // resolved by ParseFile, nil from Parse
	Pos      Position
}

type Module struct {
	Name       string
	Path       string // explicit path, empty if auto-discovered
	Optional   bool
	Justfile   *Justfile // resolved by ParseFile, nil from Parse
	Attributes []*Attribute
	Pos        Position
}

type Recipe struct {
	Name         string
	Parameters   []*Parameter
	Dependencies []*Dependency
	Body         []*RecipeLine
	Attributes   []*Attribute
	Shebang      string // e.g. "#!/usr/bin/env bash", empty if none
	Quiet        bool   // prefixed with @
	Comment      string
	Pos          Position
}

type Parameter struct {
	Name     string
	Default  Expression // nil if no default
	Export   bool       // prefixed with $
	Variadic string     // "", "*", or "+"
	Pos      Position
}

type Dependency struct {
	Name       string
	Arguments  []Expression
	Subsequent bool // after &&
	Pos        Position
}

type RecipeLine struct {
	Fragments   []Fragment
	Quiet       bool   // @
	NoError     bool   // -
	PrefixOrder string // "@-" or "-@" when both Quiet and NoError; empty otherwise
	Pos         Position
}

// Fragment is a piece of a recipe line — either text or an interpolation.
type Fragment interface {
	fragmentNode()
	GetPos() Position
}

type TextFragment struct {
	Value string
	Pos   Position
}

func (*TextFragment) fragmentNode()      {}
func (t *TextFragment) GetPos() Position { return t.Pos }

type InterpolationFragment struct {
	Expression Expression
	Pos        Position
}

func (*InterpolationFragment) fragmentNode()      {}
func (i *InterpolationFragment) GetPos() Position { return i.Pos }

// Expression represents any expression in a justfile.
type Expression interface {
	exprNode()
	GetPos() Position
}

type StringLiteral struct {
	Value string
	Kind  string // StringKind* constant identifying the string type
	Pos   Position
}

func (*StringLiteral) exprNode()          {}
func (s *StringLiteral) GetPos() Position { return s.Pos }

type Variable struct {
	Name string
	Pos  Position
}

func (*Variable) exprNode()          {}
func (v *Variable) GetPos() Position { return v.Pos }

type Concatenation struct {
	Left  Expression
	Right Expression
	Pos   Position
}

func (*Concatenation) exprNode()          {}
func (c *Concatenation) GetPos() Position { return c.Pos }

type PathJoin struct {
	Left  Expression
	Right Expression
	Pos   Position
}

func (*PathJoin) exprNode()          {}
func (p *PathJoin) GetPos() Position { return p.Pos }

type FunctionCall struct {
	Name      string
	Arguments []Expression
	Pos       Position
}

func (*FunctionCall) exprNode()          {}
func (f *FunctionCall) GetPos() Position { return f.Pos }

type Conditional struct {
	Condition Comparison
	Then      Expression
	Otherwise Expression
	Pos       Position
}

func (*Conditional) exprNode()          {}
func (c *Conditional) GetPos() Position { return c.Pos }

type BacktickExpr struct {
	Command  string
	Indented bool
	Pos      Position
}

func (*BacktickExpr) exprNode()          {}
func (b *BacktickExpr) GetPos() Position { return b.Pos }

type ParenExpr struct {
	Inner Expression
	Pos   Position
}

func (*ParenExpr) exprNode()          {}
func (p *ParenExpr) GetPos() Position { return p.Pos }

type LogicalOp struct {
	Left     Expression
	Right    Expression
	Operator string // "&&", "||"
	Pos      Position
}

func (*LogicalOp) exprNode()          {}
func (l *LogicalOp) GetPos() Position { return l.Pos }

type Comparison struct {
	Left     Expression
	Right    Expression
	Operator string // "==", "!=", "=~", "!~"
	Pos      Position
}

func (*Comparison) exprNode()          {}
func (c *Comparison) GetPos() Position { return c.Pos }

type Attribute struct {
	Name      string
	Arguments []AttributeArg
	Pos       Position
}

type AttributeArg struct {
	Key   string // empty for positional args
	Value string
}
