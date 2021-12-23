package ast

import (
	"io"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Program struct {
	Pos lexer.Position

	Rules []Rule `parser:"@@*"`
}

type Rule struct {
	Pos lexer.Position

	Head Atom   `parser:"@@ ':-'"`
	Body []Atom `parser:"@@ (',' @@)*"`
}

type Atom struct {
	Pos lexer.Position

	Name      string     `parser:"@Ident"`
	Variables []Variable `parser:"'(' @@ (',' @@)* ')'"`
}

type Variable struct {
	Pos lexer.Position

	NameTuple []*Variable `parser:"('(' @@ (',' @@)* ')')|"`
	Name      string      `parser:"@Ident"`
}

var (
	lex = lexer.MustSimple([]lexer.Rule{
		{"Ident", `[a-zA-Z]([a-zA-Z0-9])*\b`, nil},
		{"Oper", `[()]|:-`, nil},
		{"Delim", `[,]`, nil},
		{"EOL", `\\n+`, nil},
		{"whitespace", `\s+`, nil},
	})

	parser = participle.MustBuild(&Program{}, participle.Lexer(lex))
)

func Parse(r io.Reader) (*Program, error) {
	program := &Program{}
	err := parser.Parse("", r, program)
	if err != nil {
		return nil, err
	}
	return program, nil
}
