package cmd

import (
	"bytes"

	"github.com/rithvikp/dedalus/ast"
	"github.com/rithvikp/dedalus/engine"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "dedalus",
		Run: run,
	}
)

// Execute starts the program.
func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	program := `out(a,b,c) :- in1(a,b), in2(b,c)`
	p, _ := ast.Parse(bytes.NewReader([]byte(program)))
	r := engine.NewRunner(p)
	//fmt.Printf("%+v", r)
	r.Step()
}
