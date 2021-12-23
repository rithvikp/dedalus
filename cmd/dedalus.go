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
out(a,b,c,L1,S) :- out(a,b,c,L1,T), choose((a,b,c),S)

.
in1("a","b",L1,0).
in1("f","b",L1,0).
in2("b","c",L1,0).
in2("a","b",L1,0).
`

	//program := `out(a,a,L1,T) :- in1(a,a,L1,T), in2(a,a,L1,T)

	//.
	//in1("a","a",L1,0).
	//in1("f","b",L1,0).
	//in2("a","a",L1,0).
	//in2("a","b",L1,0).
	//`

	p, err := ast.Parse(bytes.NewReader([]byte(program)))
	if err != nil {
		fmt.Printf("Unable to parse your program: %v\n", err)
		os.Exit(1)
	}

	r, err := engine.NewRunner(p)
	if err != nil {
		fmt.Printf("Unable to run your program: %v\n", err)
		os.Exit(1)
	}
	r.Step()
}
