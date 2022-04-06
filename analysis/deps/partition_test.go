package deps

import (
	"testing"
)

// TODO: This test function currently just outputs to stdout for manual inspection. This will be changed soon.
func TestDistributionPolicy(t *testing.T) {
	s := stateFromProgram(t, p6)
	_ = s
	//cds := CDs(s)

	//for cdMap, deps := range cds {
	//fmt.Printf("\n============== %s -> %s ==============\n", cdMap.Dom.ID(), cdMap.Codom.ID())
	//for _, dep := range deps.Elems() {
	//fmt.Printf("\n%v\n\n", dep)
	//}
	//fmt.Println("Num FDs:", len(deps.Elems()))
	//fmt.Printf("\n============================\n")
	//}
}
