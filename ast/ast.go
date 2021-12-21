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
	Pos       lexer.Position
	Name      string     `parser:"@Ident"`
	Variables []Variable `parser:"'(' (@@ ',')+"`
	Loc       string     `parser:"@Loc ','"`
	Time      string     `parser:"@Time')'"`
}

type Variable struct {
	Pos  lexer.Position
	Name string `parser:"@Ident"`
}

var (
	lex = lexer.MustSimple([]lexer.Rule{
		{"Ident", `[a-z]([a-z0-9])*\b`, nil},
		{"Loc", `L([0-9])+\b`, nil},
		{"Time", `[A-Z]\b`, nil},
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
