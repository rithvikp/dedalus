package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rithvikp/dedalus/ast"
	"github.com/rithvikp/dedalus/engine"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:  "dedalus",
		Run:  run,
		Args: cobra.ExactArgs(1),
	}
)

// Execute starts the program.
func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Printf("Unable to read the source file: %v\n", err)
		os.Exit(1)
	}

	p, err := ast.Parse(f)
	if err != nil {
		fmt.Printf("Unable to parse your program: %v\n", err)
		os.Exit(1)
	}

	r, err := engine.NewRunner(p)
	if err != nil {
		fmt.Printf("Unable to run your program: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("<=== Ready to begin execution ===>")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		in := scanner.Text()

		tokens := strings.Split(in, " ")
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "s", "step":
			r.Step()

		case "p", "print":
			if len(tokens) != 2 {
				fmt.Println("The print command requires on additional argument: the name of the relation to be printed")
				continue
			}
			fmt.Println()
			err := r.PrintRelation(tokens[1])
			if err != nil {
				fmt.Printf("Unable to print relation %s: %v\n", tokens[1], err)
				continue
			}
			fmt.Println()

		case "h", "help":
			fmt.Println("TODO: help page")

		}

	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Unable to read input: %v\n", err)
	}
}
