package deps

import (
	"fmt"
	"testing"
)

var p6 = `add("1", "2", "3").
out(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)
`

// TODO: This test function currently just outputs to stdout for manual inspection. This will be changed soon.
func TestCDs(t *testing.T) {
	s := stateFromProgram(t, p6)
	cds := CDs(s)

	for cdMap, deps := range cds {
		fmt.Printf("\n============== %s -> %s ==============\n", cdMap.Dom.ID(), cdMap.Codom.ID())
		for _, dep := range deps.Elems() {
			fmt.Printf("\n%v\n\n", dep)
		}
		fmt.Println("Num FDs:", len(deps.Elems()))
		fmt.Printf("\n============================\n")
	}
}
