package deps

import (
	"fmt"
	"sort"
)

// TODO: Some of this functionality is duplicated by the runtime's internal expression system.

type Function struct {
	DomainDim     int
	CodomainDim   int
	domainToInput []Set[int] // Map from domain indices to expression input indices (var-attr substitutions).
	inputDim      int
	exp           Expression
}

type Expression interface {
	Eval(input []int) int
}

func (f *Function) Eval(x []int) int {
	input := make([]int, f.inputDim)
	for i, mapping := range f.domainToInput {
		for j := range mapping {
			input[j] = x[i]
		}
	}
	fmt.Println(input, f.domainToInput)
	return f.exp.Eval(input)
}

func (f *Function) MergeDomain(indices []int) {
	if len(indices) == 0 {
		return
	}
	f.DomainDim -= len(indices) - 1

	sort.Slice(indices, func(i, j int) bool { return i < j })
	min := indices[0]

	// TODO: Add validation of indices
	mapping := f.domainToInput[min]

	for _, i := range indices {
		mapping.Union(f.domainToInput[i])
	}

	var dToI []Set[int]
	i := 0
	for j, m := range f.domainToInput {
		if i >= len(indices) || indices[i] != j || indices[i] == min {
			dToI = append(dToI, m)

			if i < len(indices) && indices[i] == min {
				i++
			}
		} else {
			i++
		}
	}
	f.domainToInput = dToI
}

func IdentityFunc() Function {
	return ExpressionFunc(IdentityExp(0), 1)
}

func ExpressionFunc(exp Expression, domainDim int) Function {
	dToI := make([]Set[int], domainDim)
	for i := 0; i < domainDim; i++ {
		dToI[i] = Set[int]{i: true}
	}

	return Function{
		DomainDim:     domainDim,
		CodomainDim:   1,
		domainToInput: dToI,
		inputDim:      domainDim,
		exp:           exp,
	}
}

func funcEqual(a, b Function) bool {
	if a.DomainDim != b.DomainDim {
		return false
	} else if a.CodomainDim != b.CodomainDim {
		return false
	}

	// This is a very basic heuristic for "equality" of functions
	values := []int{0, 1, 31, 100}
	for _, v := range values {
		input := make([]int, a.DomainDim)
		for i := 0; i < len(input); i++ {
			input[i] = v + i
		}

		if a.Eval(input) != b.Eval(input) {
			return false
		}

		for i := 0; i < len(input); i++ {
			input[i] = -input[i]
		}
		if a.Eval(input) != b.Eval(input) {
			return false
		}
	}

	return true
}

type binOp struct {
	e1 Expression
	e2 Expression
	op string
}

type number int

type identity struct {
	index int
}

func (b binOp) Eval(input []int) int {
	switch b.op {
	case "+":
		return b.e1.Eval(input) + b.e2.Eval(input)
	case "-":
		return b.e1.Eval(input) - b.e2.Eval(input)
	case "*":
		return b.e1.Eval(input) * b.e2.Eval(input)
	case "%":
		return b.e1.Eval(input) % b.e2.Eval(input)
	}

	return 0
}

func (n number) Eval(input []int) int {
	return int(n)
}

func (i identity) Eval(input []int) int {
	return input[i.index]
}

func AddExp(right, left Expression) Expression {
	return binOp{
		e1: right,
		e2: left,
		op: "+",
	}
}

func IdentityExp(index int) Expression {
	return identity{index: index}
}
