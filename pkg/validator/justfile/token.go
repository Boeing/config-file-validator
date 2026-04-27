package gojust

import "fmt"

type Position struct {
	Line   int
	Column int
	Offset int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// StringKind identifies the type of string literal in the source.
const (
	StringKindQuoted           = "string"
	StringKindRaw              = "raw string"
	StringKindIndentedQuoted   = "indented string"
	StringKindIndentedRaw      = "indented raw string"
	StringKindBacktick         = "backtick"
	StringKindIndentedBacktick = "indented backtick"
	StringKindShellExpanded    = "shell-expanded string"
	StringKindShellExpandedRaw = "shell-expanded raw string"
	StringKindFormat           = "format string"
)

type tokenType int

const (
	// Literals
	tokenIdentifier tokenType = iota
	tokenString
	tokenRawString
	tokenIndentedString
	tokenIndentedRawString
	tokenBacktick
	tokenIndentedBacktick
	tokenShellExpandedString
	tokenShellExpandedRawString
	tokenFormatString

	// Keywords
	tokenAlias
	tokenExport
	tokenUnexport
	tokenImport
	tokenMod
	tokenSet
	tokenIf
	tokenElse
	tokenTrue
	tokenFalse
	tokenEager

	// Operators and punctuation
	tokenAssign        // :=
	tokenEquals        // ==
	tokenNotEquals     // !=
	tokenRegexMatch    // =~
	tokenRegexMismatch // !~
	tokenPlus          // +
	tokenSlash         // /
	tokenAnd           // &&
	tokenOr            // ||
	tokenAt            // @
	tokenStar          // *
	tokenColon         // :
	tokenComma         // ,
	tokenDollar        // $
	tokenQuestion      // ?
	tokenParamAssign   // =
	tokenBang          // !
	tokenLParen        // (
	tokenRParen        // )
	tokenLBracket      // [
	tokenRBracket      // ]
	tokenLBrace        // {
	tokenRBrace        // }
	tokenInterpolStart // {{
	tokenInterpolEnd   // }}
	tokenAtDash        // @-
	tokenDashAt        // -@
	tokenDash          // -

	// Whitespace and structure
	tokenNewline
	tokenIndent
	tokenDedent
	tokenComment
	tokenText // opaque text in recipe bodies

	tokenEOF
)

var tokenNames = map[tokenType]string{
	tokenIdentifier:             "identifier",
	tokenString:                 "string",
	tokenRawString:              "raw string",
	tokenIndentedString:         "indented string",
	tokenIndentedRawString:      "indented raw string",
	tokenBacktick:               "backtick",
	tokenIndentedBacktick:       "indented backtick",
	tokenShellExpandedString:    "shell-expanded string",
	tokenShellExpandedRawString: "shell-expanded raw string",
	tokenFormatString:           "format string",
	tokenAlias:                  "'alias'",
	tokenExport:                 "'export'",
	tokenUnexport:               "'unexport'",
	tokenImport:                 "'import'",
	tokenMod:                    "'mod'",
	tokenSet:                    "'set'",
	tokenIf:                     "'if'",
	tokenElse:                   "'else'",
	tokenTrue:                   "'true'",
	tokenFalse:                  "'false'",
	tokenEager:                  "'eager'",
	tokenAssign:                 "':='",
	tokenEquals:                 "'=='",
	tokenNotEquals:              "'!='",
	tokenRegexMatch:             "'=~'",
	tokenRegexMismatch:          "'!~'",
	tokenPlus:                   "'+'",
	tokenSlash:                  "'/'",
	tokenAnd:                    "'&&'",
	tokenOr:                     "'||'",
	tokenAt:                     "'@'",
	tokenStar:                   "'*'",
	tokenColon:                  "':'",
	tokenComma:                  "','",
	tokenDollar:                 "'$'",
	tokenQuestion:               "'?'",
	tokenParamAssign:            "'='",
	tokenBang:                   "'!'",
	tokenLParen:                 "'('",
	tokenRParen:                 "')'",
	tokenLBracket:               "'['",
	tokenRBracket:               "']'",
	tokenLBrace:                 "'{'",
	tokenRBrace:                 "'}'",
	tokenInterpolStart:          "'{{'",
	tokenInterpolEnd:            "'}}'",
	tokenAtDash:                 "'@-'",
	tokenDashAt:                 "'-@'",
	tokenDash:                   "'-'",
	tokenNewline:                "newline",
	tokenIndent:                 "indent",
	tokenDedent:                 "dedent",
	tokenComment:                "comment",
	tokenText:                   "text",
	tokenEOF:                    "EOF",
}

func (t tokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("token(%d)", int(t))
}

type token struct {
	Type  tokenType
	Value string
	Pos   Position
}

func (t token) String() string {
	return fmt.Sprintf("%s %s at %s", t.Type, t.Value, t.Pos)
}

// tokenTypeToStringKind maps token types to their public StringKind constants.
var tokenTypeToStringKind = map[tokenType]string{
	tokenString:                 StringKindQuoted,
	tokenRawString:              StringKindRaw,
	tokenIndentedString:         StringKindIndentedQuoted,
	tokenIndentedRawString:      StringKindIndentedRaw,
	tokenBacktick:               StringKindBacktick,
	tokenIndentedBacktick:       StringKindIndentedBacktick,
	tokenShellExpandedString:    StringKindShellExpanded,
	tokenShellExpandedRawString: StringKindShellExpandedRaw,
	tokenFormatString:           StringKindFormat,
}
