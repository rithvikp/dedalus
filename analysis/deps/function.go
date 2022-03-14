package deps

import (
	"fmt"
	"sort"

	"golang.org/x/exp/slices"
)

// TODO: Some of this functionality is duplicated by the runtime's internal expression system.

type inputTransformation struct {
	inputIndices []int
	f            Function
}

type Function struct {
	DomainDim            int
	CodomainDim          int
	domainToInput        []Set[int]                  // Map from domain indices to expression input indices (var-attr substitutions).
	inputTransformations map[int]inputTransformation // Map from input indices to corresponding functions that should be applied before evaluating the function.
	inputDim             int
	exp                  Expression
}

type Expression interface {
	Eval(input []int) int
}

func (f Function) String() string {
	return fmt.Sprintf("{ Dom: %d, Codom: %d, Exp: %+v }", f.DomainDim, f.CodomainDim, f.exp)
}

func (f *Function) Clone() Function {
	g := *f

	g.domainToInput = make([]Set[int], len(f.domainToInput))
	for i, s := range f.domainToInput {
		g.domainToInput[i] = s.Clone()
	}

	g.inputTransformations = map[int]inputTransformation{}
	for i, it := range f.inputTransformations {
		g.inputTransformations[i] = inputTransformation{
			inputIndices: slices.Clone(it.inputIndices),
			f:            it.f.Clone(),
		}
	}

	return g
}

func (f *Function) Eval(x []int) int {
	input := make([]int, f.inputDim)
	for i, mapping := range f.domainToInput {
		for j := range mapping {
			input[j] = x[i]
		}
	}

	for i := range input {
		it, ok := f.inputTransformations[i]
		if !ok {
			continue
		}

		transformIn := make([]int, len(it.inputIndices))
		for j, k := range it.inputIndices {
			// TODO: Handle dependent transformations
			transformIn[j] = input[k]
		}
		input[i] = it.f.Eval(transformIn)
	}

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

func (f *Function) AddToDomain(n int) {
	for i := 0; i < n; i++ {
		f.domainToInput = append(f.domainToInput, Set[int]{f.inputDim: true})
		f.DomainDim++
		f.inputDim++
	}
}

func (f *Function) FunctionSubstitution(substIndex int, domIndices []int, g Function) {
	f.DomainDim -= 1

	inputIndices := make([]int, len(domIndices))
	for i, index := range domIndices {
		inputIndices[i] = f.domainToInput[index].Elems()[0]
	}

	for i := range f.domainToInput[substIndex] {
		if _, ok := f.inputTransformations[i]; ok {
			// TODO: handle composition
			panic("FD composition is not currently handled")
		} else {
			f.inputTransformations[i] = inputTransformation{inputIndices: inputIndices, f: g}
		}
	}
	f.domainToInput = slices.Delete(f.domainToInput, substIndex, substIndex+1)
}

func IdentityFunc() Function {
	return ExprFunc(IdentityExp(0), 1)
}

func ConstFunc(val int) Function {
	return ExprFunc(number(val), 0)
}

func ExprFunc(exp Expression, domainDim int) Function {
	dToI := make([]Set[int], domainDim)
	for i := 0; i < domainDim; i++ {
		dToI[i] = Set[int]{i: true}
	}

	return Function{
		DomainDim:            domainDim,
		CodomainDim:          1,
		domainToInput:        dToI,
		inputTransformations: map[int]inputTransformation{},
		inputDim:             domainDim,
		exp:                  exp,
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

func SubExp(right, left Expression) Expression {
	return binOp{
		e1: right,
		e2: left,
		op: "-",
	}
}

func IdentityExp(index int) Expression {
	return identity{index: index}
}
