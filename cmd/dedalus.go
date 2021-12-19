package cmd

import (
	"bytes"
	"fmt"

	"github.com/rithvikp/dedalus/ast"
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
	program := `out(a,b) :- in1(a,b), in2(b,c)`
	p, _ := ast.Parse(bytes.NewReader([]byte(program)))
	fmt.Printf("%+v", p)
}
