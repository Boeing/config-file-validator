package gojust

import (
	"fmt"
	"strings"
)

type parser struct {
	tokens      []token
	pos         int
	file        string
	lastComment string // most recent comment, cleared on non-comment/newline tokens
}

func newParser(tokens []token, file string) *parser {
	return &parser{tokens: tokens, file: file}
}

func (p *parser) parse() (*Justfile, error) {
	jf := &Justfile{File: p.file}

	for !p.atEnd() {
		if p.check(tokenNewline) {
			p.advance()
			continue
		}
		if p.check(tokenComment) {
			val := p.current().Value
			jf.Comments = append(jf.Comments, &Comment{
				Value: val,
				Pos:   p.current().Pos,
			})
			// Strip leading # and space for doc comments
			if len(val) > 1 && val[0] == '#' {
				p.lastComment = strings.TrimLeft(val[1:], " ")
			} else {
				p.lastComment = val
			}
			p.advance()
			continue
		}

		if err := p.parseItem(jf); err != nil {
			return nil, err
		}
	}

	return jf, nil
}

func (p *parser) parseItem(jf *Justfile) error {
	// Attributes
	var attrs []*Attribute
	for p.check(tokenLBracket) {
		attr, err := p.parseAttributes()
		if err != nil {
			return err
		}
		attrs = append(attrs, attr...)
		p.skipNewlines()
	}

	// Non-recipe items clear the pending comment
	switch {
	case p.check(tokenAlias):
		p.lastComment = ""
		return p.parseAlias(jf, attrs)
	case p.check(tokenExport), p.check(tokenEager):
		p.lastComment = ""
		return p.parsePrefixedAssignment(jf)
	case p.check(tokenUnexport):
		p.lastComment = ""
		return p.parseUnexport(jf)
	case p.check(tokenImport):
		// 'import' followed by ':' is a recipe named 'import', not an import statement
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == tokenColon {
			return p.parseRecipe(jf, attrs)
		}
		p.lastComment = ""
		return p.parseImport(jf)
	case p.check(tokenMod):
		p.lastComment = ""
		return p.parseModule(jf, attrs)
	case p.check(tokenSet):
		p.lastComment = ""
		return p.parseSetting(jf)
	case p.check(tokenIdentifier):
		return p.parseAssignmentOrRecipe(jf, attrs)
	case p.check(tokenAt):
		return p.parseRecipe(jf, attrs)
	default:
		return p.errorf("unexpected %s", p.current().Type)
	}
}

func (p *parser) parseAlias(jf *Justfile, attrs []*Attribute) error {
	pos := p.current().Pos
	p.advance() // skip 'alias'

	name, err := p.expectIdentifier()
	if err != nil {
		return err
	}

	if err := p.expect(tokenAssign); err != nil {
		return err
	}

	target, err := p.expectIdentifier()
	if err != nil {
		return err
	}

	jf.Aliases = append(jf.Aliases, &Alias{
		Name:       name,
		Target:     target,
		Attributes: attrs,
		Pos:        pos,
	})
	return nil
}

// parsePrefixedAssignment handles assignment prefixes: export, eager, or both.
func (p *parser) parsePrefixedAssignment(jf *Justfile) error {
	pos := p.current().Pos

	var export, eager bool
	for p.check(tokenExport) || p.check(tokenEager) {
		if p.check(tokenExport) {
			export = true
		} else {
			eager = true
		}
		p.advance()
	}

	name, err := p.expectIdentifier()
	if err != nil {
		return err
	}

	if err := p.expect(tokenAssign); err != nil {
		return err
	}

	value, err := p.parseExpression()
	if err != nil {
		return err
	}

	jf.Assignments = append(jf.Assignments, &Assignment{
		Name:   name,
		Value:  value,
		Export: export,
		Eager:  eager,
		Pos:    pos,
	})
	return nil
}

func (p *parser) parseUnexport(jf *Justfile) error {
	pos := p.current().Pos
	p.advance() // skip 'unexport'

	name, err := p.expectIdentifier()
	if err != nil {
		return err
	}

	jf.Unexports = append(jf.Unexports, &Unexport{
		Name: name,
		Pos:  pos,
	})
	return nil
}

func (p *parser) parseImport(jf *Justfile) error {
	pos := p.current().Pos
	p.advance() // skip 'import'

	optional := false
	if p.check(tokenQuestion) {
		optional = true
		p.advance()
	}

	path, err := p.expectString()
	if err != nil {
		return err
	}

	jf.Imports = append(jf.Imports, &Import{
		Path:     path,
		Optional: optional,
		Pos:      pos,
	})
	return nil
}

func (p *parser) parseModule(jf *Justfile, attrs []*Attribute) error {
	pos := p.current().Pos
	p.advance() // skip 'mod'

	optional := false
	if p.check(tokenQuestion) {
		optional = true
		p.advance()
	}

	name, err := p.expectIdentifier()
	if err != nil {
		return err
	}

	var path string
	if p.check(tokenString) || p.check(tokenRawString) {
		path = p.current().Value
		p.advance()
	}

	jf.Modules = append(jf.Modules, &Module{
		Name:       name,
		Path:       path,
		Optional:   optional,
		Attributes: attrs,
		Pos:        pos,
	})
	return nil
}

func (p *parser) parseSetting(jf *Justfile) error {
	pos := p.current().Pos
	p.advance() // skip 'set'

	// Setting names can be keywords like 'export'
	name, err := p.expectAnyIdentifier()
	if err != nil {
		return err
	}

	setting := &Setting{Name: name, Pos: pos}

	// Setting with no value (boolean true)
	if p.check(tokenNewline) || p.check(tokenEOF) || p.check(tokenComment) {
		t := true
		setting.Value.Bool = &t
		jf.Settings = append(jf.Settings, setting)
		return nil
	}

	if err := p.expect(tokenAssign); err != nil {
		return err
	}

	// Boolean value
	if p.check(tokenTrue) || p.check(tokenFalse) {
		b := p.current().Type == tokenTrue
		setting.Value.Bool = &b
		p.advance()
		jf.Settings = append(jf.Settings, setting)
		return nil
	}

	// List value [...]
	if p.check(tokenLBracket) {
		p.advance() // skip [
		var items []string
		for !p.check(tokenRBracket) {
			s, err := p.expectString()
			if err != nil {
				return err
			}
			items = append(items, s)
			if p.check(tokenComma) {
				p.advance()
			}
		}
		if err := p.expect(tokenRBracket); err != nil {
			return err
		}
		setting.Value.List = items
		jf.Settings = append(jf.Settings, setting)
		return nil
	}

	// String value
	s, err := p.expectString()
	if err != nil {
		return err
	}
	setting.Value.String = &s
	jf.Settings = append(jf.Settings, setting)
	return nil
}

func (p *parser) parseAssignmentOrRecipe(jf *Justfile, attrs []*Attribute) error {
	// Look ahead to determine if this is an assignment, function def, or recipe.
	// Use peekAt to avoid mutating parser state.
	if p.peekAt(1).Type == tokenAssign {
		return p.parseAssignment(jf, attrs)
	}

	// Function definition: name(params) := expression
	if p.peekAt(1).Type == tokenLParen {
		depth := 1
		i := 2
		for depth > 0 {
			tok := p.peekAt(i)
			switch tok.Type {
			case tokenEOF:
				depth = 0
			case tokenLParen:
				depth++
			case tokenRParen:
				depth--
			default:
			}
			i++
		}
		if p.peekAt(i).Type == tokenAssign {
			return p.parseFunctionDef(jf)
		}
	}

	return p.parseRecipe(jf, attrs)
}

func (p *parser) parseAssignment(jf *Justfile, attrs []*Attribute) error {
	p.lastComment = ""
	pos := p.current().Pos
	name := p.current().Value
	p.advance() // skip name

	if err := p.expect(tokenAssign); err != nil {
		return err
	}

	value, err := p.parseExpression()
	if err != nil {
		return err
	}

	private := false
	for _, a := range attrs {
		if a.Name == "private" {
			private = true
		}
	}

	jf.Assignments = append(jf.Assignments, &Assignment{
		Name:    name,
		Value:   value,
		Private: private,
		Pos:     pos,
	})
	return nil
}

func (p *parser) parseFunctionDef(jf *Justfile) error {
	pos := p.current().Pos
	name := p.current().Value
	p.advance() // skip name
	p.advance() // skip (

	var params []string
	for !p.check(tokenRParen) && !p.atEnd() {
		param, err := p.expectIdentifier()
		if err != nil {
			return err
		}
		params = append(params, param)
		if p.check(tokenComma) {
			p.advance()
		}
	}
	if err := p.expect(tokenRParen); err != nil {
		return err
	}
	if err := p.expect(tokenAssign); err != nil {
		return err
	}

	// Function definitions can span multiple lines
	p.skipNewlines()
	value, err := p.parseExpression()
	if err != nil {
		return err
	}

	jf.Assignments = append(jf.Assignments, &Assignment{
		Name:       name,
		Value:      value,
		Parameters: params,
		Pos:        pos,
	})
	return nil
}

func (p *parser) parseRecipe(jf *Justfile, attrs []*Attribute) error {
	pos := p.current().Pos

	quiet := false
	if p.check(tokenAt) {
		quiet = true
		p.advance() // skip @
	}

	// Grab preceding comment as doc and clear it
	comment := p.lastComment
	p.lastComment = ""

	name, err := p.expectIdentifierOrKeyword()
	if err != nil {
		return err
	}

	// Parameters
	var params []*Parameter
	for !p.check(tokenColon) && !p.check(tokenNewline) && !p.check(tokenEOF) {
		param, err := p.parseParameter()
		if err != nil {
			return err
		}
		params = append(params, param)
	}

	if err := p.expect(tokenColon); err != nil {
		return err
	}

	// Dependencies
	var deps []*Dependency
	subsequent := false
	for !p.check(tokenNewline) && !p.check(tokenEOF) && !p.check(tokenComment) {
		if p.check(tokenAnd) {
			subsequent = true
			p.advance()
		}
		dep, err := p.parseDependency(subsequent)
		if err != nil {
			return err
		}
		deps = append(deps, dep)
	}

	// Skip to body
	p.skipNewlines()

	// Recipe body
	var body []*RecipeLine
	if p.check(tokenIndent) {
		p.advance()
		for !p.check(tokenDedent) && !p.atEnd() {
			if p.check(tokenNewline) {
				p.advance()
				continue
			}
			line, err := p.parseRecipeLine()
			if err != nil {
				return err
			}
			body = append(body, line)
		}
		if p.check(tokenDedent) {
			p.advance()
		}
	}

	jf.Recipes = append(jf.Recipes, &Recipe{
		Name:         name,
		Parameters:   params,
		Dependencies: deps,
		Body:         body,
		Attributes:   attrs,
		Shebang:      extractShebang(body),
		Quiet:        quiet,
		Comment:      comment,
		Pos:          pos,
	})
	return nil
}

func (p *parser) parseParameter() (*Parameter, error) {
	pos := p.current().Pos
	param := &Parameter{Pos: pos}

	// Variadic
	if p.check(tokenStar) || p.check(tokenPlus) {
		param.Variadic = p.current().Value
		p.advance()
	}

	// Export
	if p.check(tokenDollar) {
		param.Export = true
		p.advance()
	}

	name, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	param.Name = name

	// Default value — only accepts a single value, not a full expression,
	// because + and / in parameter context are ambiguous with variadic
	// and path operators.
	if p.check(tokenParamAssign) {
		p.advance()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		param.Default = val
	}

	return param, nil
}

func (p *parser) parseDependency(subsequent bool) (*Dependency, error) {
	pos := p.current().Pos

	// Dependency with arguments: (name args...)
	if p.check(tokenLParen) {
		p.advance()
		name, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}
		var args []Expression
		for !p.check(tokenRParen) && !p.atEnd() {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, expr)
		}
		if err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
		return &Dependency{Name: name, Arguments: args, Subsequent: subsequent, Pos: pos}, nil
	}

	name, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	return &Dependency{Name: name, Subsequent: subsequent, Pos: pos}, nil
}

func (p *parser) parseRecipeLine() (*RecipeLine, error) {
	pos := p.current().Pos
	line := &RecipeLine{Pos: pos}

	// Line prefix
	if p.check(tokenAtDash) {
		line.Quiet = true
		line.NoError = true
		line.PrefixOrder = "@-"
		p.advance()
	} else if p.check(tokenDashAt) {
		line.Quiet = true
		line.NoError = true
		line.PrefixOrder = "-@"
		p.advance()
	} else if p.check(tokenAt) {
		line.Quiet = true
		p.advance()
	} else if p.check(tokenDash) {
		line.NoError = true
		p.advance()
	}

	// Fragments
	for !p.check(tokenNewline) && !p.check(tokenDedent) && !p.atEnd() {
		if p.check(tokenInterpolStart) {
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if err := p.expect(tokenInterpolEnd); err != nil {
				return nil, err
			}
			line.Fragments = append(line.Fragments, &InterpolationFragment{
				Expression: expr,
				Pos:        pos,
			})
		} else if p.check(tokenText) {
			line.Fragments = append(line.Fragments, &TextFragment{
				Value: p.current().Value,
				Pos:   p.current().Pos,
			})
			p.advance()
		} else {
			return nil, p.errorf("unexpected %s in recipe body", p.current().Type)
		}
	}

	if p.check(tokenNewline) {
		p.advance()
	}

	return line, nil
}

func (p *parser) parseAttributes() ([]*Attribute, error) {
	if err := p.expect(tokenLBracket); err != nil {
		return nil, err
	}

	var attrs []*Attribute
	for !p.check(tokenRBracket) && !p.atEnd() {
		attr, err := p.parseAttribute()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
		if p.check(tokenComma) {
			p.advance()
		}
	}

	if err := p.expect(tokenRBracket); err != nil {
		return nil, err
	}
	return attrs, nil
}

func (p *parser) parseAttribute() (*Attribute, error) {
	pos := p.current().Pos
	// Attribute names can be identifiers or keywords used as identifiers
	name, err := p.expectAnyIdentifier()
	if err != nil {
		return nil, err
	}

	attr := &Attribute{Name: name, Pos: pos}

	// [name(args...)]
	if p.check(tokenLParen) {
		p.advance()
		for !p.check(tokenRParen) && !p.atEnd() {
			arg, err := p.parseAttributeArg()
			if err != nil {
				return nil, err
			}
			attr.Arguments = append(attr.Arguments, arg)
			if p.check(tokenComma) {
				p.advance()
			}
		}
		if err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
	}

	// [name: 'value'] shorthand
	if p.check(tokenColon) {
		p.advance()
		val, err := p.expectString()
		if err != nil {
			return nil, err
		}
		attr.Arguments = []AttributeArg{{Value: val}}
	}

	return attr, nil
}

func (p *parser) parseAttributeArg() (AttributeArg, error) {
	// Check for key=value
	if p.check(tokenIdentifier) && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == tokenParamAssign {
		key := p.current().Value
		p.advance() // key
		p.advance() // =
		val, err := p.expectString()
		if err != nil {
			return AttributeArg{}, err
		}
		return AttributeArg{Key: key, Value: val}, nil
	}

	// Positional string arg
	if p.check(tokenString) || p.check(tokenRawString) {
		val := p.current().Value
		p.advance()
		return AttributeArg{Value: val}, nil
	}

	// Positional identifier arg
	if p.check(tokenIdentifier) {
		val := p.current().Value
		p.advance()
		return AttributeArg{Value: val}, nil
	}

	return AttributeArg{}, p.errorf("expected attribute argument, found %s", p.current().Type)
}

// Expression parsing

func (p *parser) parseExpression() (Expression, error) {
	// Expressions can start with / for absolute paths
	if p.check(tokenSlash) {
		pos := p.current().Pos
		p.advance()
		right, err := p.parseLogicalOr()
		if err != nil {
			return nil, err
		}
		return &PathJoin{
			Left:  &StringLiteral{Value: "", Kind: StringKindQuoted, Pos: pos},
			Right: right,
			Pos:   pos,
		}, nil
	}
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() (Expression, error) {
	left, err := p.parseLogicalAnd()
	if err != nil {
		return nil, err
	}

	for p.check(tokenOr) {
		pos := p.current().Pos
		p.advance()
		right, err := p.parseLogicalAnd()
		if err != nil {
			return nil, err
		}
		left = &LogicalOp{Left: left, Right: right, Operator: "||", Pos: pos}
	}
	return left, nil
}

func (p *parser) parseLogicalAnd() (Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for p.check(tokenAnd) {
		pos := p.current().Pos
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &LogicalOp{Left: left, Right: right, Operator: "&&", Pos: pos}
	}
	return left, nil
}

func (p *parser) parseComparison() (Expression, error) {
	left, err := p.parsePathJoin()
	if err != nil {
		return nil, err
	}

	if p.check(tokenEquals) || p.check(tokenNotEquals) || p.check(tokenRegexMatch) || p.check(tokenRegexMismatch) {
		op := p.current().Value
		p.advance()
		right, err := p.parsePathJoin()
		if err != nil {
			return nil, err
		}
		return &Comparison{Left: left, Right: right, Operator: op, Pos: left.GetPos()}, nil
	}

	return left, nil
}

// parsePathJoin parses / expressions. In just, + binds tighter than /,
// and / is right-associative: a / b / c = a / (b / c).
func (p *parser) parsePathJoin() (Expression, error) {
	left, err := p.parseConcatenation()
	if err != nil {
		return nil, err
	}

	if p.check(tokenSlash) {
		pos := p.current().Pos
		p.advance()
		// Right-associative: recurse into parsePathJoin for the right side
		right, err := p.parsePathJoin()
		if err != nil {
			return nil, err
		}
		return &PathJoin{Left: left, Right: right, Pos: pos}, nil
	}
	return left, nil
}

// parseConcatenation parses + expressions. + is right-associative in just:
// a + b + c = a + (b + c).
func (p *parser) parseConcatenation() (Expression, error) {
	left, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	if p.check(tokenPlus) {
		pos := p.current().Pos
		p.advance()
		// Right-associative: recurse into parseConcatenation for the right side
		right, err := p.parseConcatenation()
		if err != nil {
			return nil, err
		}
		return &Concatenation{Left: left, Right: right, Pos: pos}, nil
	}
	return left, nil
}

func (p *parser) parseValue() (Expression, error) {
	tok := p.current()

	switch tok.Type {
	case tokenIf:
		return p.parseConditional()

	case tokenString, tokenRawString, tokenIndentedString, tokenIndentedRawString,
		tokenShellExpandedString, tokenShellExpandedRawString, tokenFormatString:
		p.advance()
		return &StringLiteral{Value: tok.Value, Kind: tokenTypeToStringKind[tok.Type], Pos: tok.Pos}, nil

	case tokenBacktick, tokenIndentedBacktick:
		p.advance()
		return &BacktickExpr{
			Command:  tok.Value,
			Indented: tok.Type == tokenIndentedBacktick,
			Pos:      tok.Pos,
		}, nil

	case tokenIdentifier:
		// Check for function call
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == tokenLParen {
			return p.parseFunctionCall()
		}
		p.advance()
		return &Variable{Name: tok.Value, Pos: tok.Pos}, nil

	case tokenLParen:
		pos := tok.Pos
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
		return &ParenExpr{Inner: expr, Pos: pos}, nil

	default:
		return nil, p.errorf("expected expression, found %s", tok.Type)
	}
}

func (p *parser) parseConditional() (Expression, error) {
	pos := p.current().Pos
	p.advance() // skip 'if'
	p.skipNewlines()

	// Parse condition: value op value
	left, err := p.parsePathJoin()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()

	// Operator
	if !p.check(tokenEquals) && !p.check(tokenNotEquals) && !p.check(tokenRegexMatch) && !p.check(tokenRegexMismatch) {
		return nil, p.errorf("expected comparison operator, found %s", p.current().Type)
	}
	op := p.current().Value
	p.advance()
	p.skipNewlines()

	right, err := p.parsePathJoin()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()

	if err := p.expect(tokenLBrace); err != nil {
		return nil, err
	}
	p.skipNewlines()
	then, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if err := p.expect(tokenRBrace); err != nil {
		return nil, err
	}
	p.skipNewlines()

	if err := p.expect(tokenElse); err != nil {
		return nil, err
	}
	p.skipNewlines()

	// else if ...
	var otherwise Expression
	if p.check(tokenIf) {
		otherwise, err = p.parseConditional()
		if err != nil {
			return nil, err
		}
	} else {
		if err := p.expect(tokenLBrace); err != nil {
			return nil, err
		}
		p.skipNewlines()
		otherwise, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipNewlines()
		if err := p.expect(tokenRBrace); err != nil {
			return nil, err
		}
	}

	return &Conditional{
		Condition: Comparison{Left: left, Right: right, Operator: op, Pos: pos},
		Then:      then,
		Otherwise: otherwise,
		Pos:       pos,
	}, nil
}

func (p *parser) parseFunctionCall() (Expression, error) {
	pos := p.current().Pos
	name := p.current().Value
	p.advance() // name
	p.advance() // (

	var args []Expression
	for !p.check(tokenRParen) && !p.atEnd() {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		if p.check(tokenComma) {
			p.advance()
		}
	}

	if err := p.expect(tokenRParen); err != nil {
		return nil, err
	}

	return &FunctionCall{Name: name, Arguments: args, Pos: pos}, nil
}

// Helper methods

func (p *parser) current() token {
	if p.pos >= len(p.tokens) {
		return token{Type: tokenEOF, Pos: Position{Line: -1, Column: -1}}
	}
	return p.tokens[p.pos]
}

func (p *parser) check(typ tokenType) bool {
	return p.current().Type == typ
}

func (p *parser) atEnd() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Type == tokenEOF
}

func (p *parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// peekAt returns the token at offset positions ahead without advancing.
func (p *parser) peekAt(offset int) token {
	pos := p.pos + offset
	if pos >= len(p.tokens) {
		return token{Type: tokenEOF, Pos: Position{Line: -1, Column: -1}}
	}
	return p.tokens[pos]
}

func (p *parser) expect(typ tokenType) error {
	if !p.check(typ) {
		return p.errorf("expected %s, found %s", typ, p.current().Type)
	}
	p.advance()
	return nil
}

func (p *parser) expectIdentifier() (string, error) {
	if !p.check(tokenIdentifier) {
		return "", p.errorf("expected identifier, found %s", p.current().Type)
	}
	val := p.current().Value
	p.advance()
	return val, nil
}

func (p *parser) expectIdentifierOrKeyword() (string, error) {
	// Some keywords can be used as recipe names
	switch p.current().Type {
	case tokenIdentifier, tokenImport:
		val := p.current().Value
		p.advance()
		return val, nil
	}
	return "", p.errorf("expected identifier, found %s", p.current().Type)
}

// expectAnyIdentifier accepts any identifier-like token including all keywords.
func (p *parser) expectAnyIdentifier() (string, error) {
	switch p.current().Type {
	case tokenIdentifier, tokenAlias, tokenExport, tokenUnexport, tokenEager, tokenImport, tokenMod,
		tokenSet, tokenIf, tokenElse, tokenTrue, tokenFalse:
		val := p.current().Value
		p.advance()
		return val, nil
	}
	return "", p.errorf("expected identifier, found %s", p.current().Type)
}

func (p *parser) expectString() (string, error) {
	switch p.current().Type {
	case tokenString, tokenRawString, tokenIndentedString, tokenIndentedRawString,
		tokenShellExpandedString, tokenShellExpandedRawString:
		val := p.current().Value
		p.advance()
		return val, nil
	}
	return "", p.errorf("expected string, found %s", p.current().Type)
}

func (p *parser) skipNewlines() {
	for p.check(tokenNewline) || p.check(tokenComment) {
		p.advance()
	}
}

func (p *parser) errorf(format string, args ...any) *ParseError {
	tok := p.current()
	return &ParseError{
		Pos:     tok.Pos,
		Message: fmt.Sprintf(format, args...),
		File:    p.file,
	}
}

// extractShebang checks if the first line of a recipe body is a shebang
// and returns it. Returns empty string if no shebang.
func extractShebang(body []*RecipeLine) string {
	if len(body) == 0 {
		return ""
	}
	line := body[0]
	if len(line.Fragments) == 0 {
		return ""
	}
	tf, ok := line.Fragments[0].(*TextFragment)
	if !ok {
		return ""
	}
	if strings.HasPrefix(tf.Value, "#!") {
		return tf.Value
	}
	return ""
}
