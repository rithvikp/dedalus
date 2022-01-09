package ast

import (
	"io"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Program struct {
	Pos lexer.Position

	Statements []Statement `parser:"@@*"`
}

type Statement struct {
	Pos lexer.Position

	Rule    *Rule    `parser:"@@ |"`
	Preload *Preload `parser:"(@@ '.') |"`
	Comment *string  `parser:"@Comment"`
}

type Rule struct {
	Pos lexer.Position

	Head HeadAtom `parser:"@@ ':-'"`
	Body []Atom   `parser:"@@ (',' @@)*"`
}

type HeadAtom struct {
	Pos lexer.Position

	Name  string     `parser:"@Ident"`
	Terms []HeadTerm `parser:"'(' @@ (',' @@)* ')'"`
}

type HeadTerm struct {
	Pos lexer.Position

	Aggregator *string  `parser:"@Ident?"`
	Variable   Variable `parser:"('<' @@ '>')|@@"` // This is a temporary workaround
}

type Atom struct {
	Pos lexer.Position

	Name      string     `parser:"@Ident"`
	Variables []Variable `parser:"'(' @@ (',' @@)* ')'"`
}

type Variable struct {
	Pos lexer.Position

	NameTuple []*Variable `parser:"('(' @@ (',' @@)* ')') |"`
	Name      string      `parser:"@Ident"`
}

type Preload struct {
	Pos lexer.Position

	Name   string         `parser:"@Ident"`
	Fields []PreloadField `parser:"'(' (@@ ',')+"`
	Loc    string         `parser:"@Ident ','"`
	Time   int            `parser:"@Int')'"`
}

type PreloadField struct {
	Pos lexer.Position

	Data string `parser:"@String"`
}

var (
	lex = lexer.MustSimple([]lexer.Rule{
		{"Ident", `[a-zA-Z]([a-zA-Z0-9_'])*`, nil},
		{"Int", `\d+`, nil},
		{"String", `"(\\"|[^"])*"`, nil},
		{"Comment", `#[^\n]*`, nil},
		{"Oper", `[()<>]|:-`, nil},
		{"Delim", `[,.]`, nil},
		{"EOL", `\\n+`, nil},
		{"whitespace", `\s+`, nil},
	})

	parser = participle.MustBuild(&Program{}, participle.Lexer(lex), participle.UseLookahead(2))
)

func Parse(r io.Reader) (*Program, error) {
	program := &Program{}
	err := parser.Parse("", r, program)
	if err != nil {
		return nil, err
	}
	return program, nil
}
