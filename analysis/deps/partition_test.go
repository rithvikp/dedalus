package deps

import (
	"fmt"
	"testing"
)

// TODO: This test function currently just outputs to stdout for manual inspection. This will be changed soon.
func TestDistributionPolicy(t *testing.T) {
	s := stateFromProgram(t, p1)
	_ = s
	policies := DistibutionPolicies(s)
	for _, p := range policies {
		fmt.Print("\n============================\n")
		for rel, dep := range p {
			fmt.Println(rel, p)
			fmt.Printf("\n%v -> %v\n\n", rel, dep)
		}
		fmt.Printf("\n============================\n")
	}
}
