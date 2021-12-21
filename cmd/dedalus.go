package cmd

import (
	"bytes"
	"fmt"
	"os"

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
	program := `out(a,b,c,L1,T) :- in1(a,b,L1,T), in2(b,c,L1,T)
out1(a,c,L1,T) :- out(a,b,c,L1,T)`
	p, err := ast.Parse(bytes.NewReader([]byte(program)))
	if err != nil {
		fmt.Printf("Unable to parse your program: %v\n", err)
		os.Exit(1)
	}

	r := engine.NewRunner(p)
	r.Step()
}
