package deps

import (
	"fmt"
	"strconv"

	"golang.org/x/exp/slices"
)

// TODO: Some of this functionality is duplicated by the runtime's internal expression system.
type Function struct {
	DomainDim   int
	CodomainDim int
	exp         Expression
}

type Expression interface {
	Eval(input []int) int
	Replace(replacements map[int]Expression) Expression
	String() string
}

func (f Function) String() string {
	return fmt.Sprintf("{ Dom: %d, Codom: %d, Exp: %v }", f.DomainDim, f.CodomainDim, f.exp)
}

func (f *Function) Clone() Function {
	g := *f

	return g
}

func (f *Function) Eval(x []int) int {
	return f.exp.Eval(x)
}

func (f *Function) MergeDomain(indices []int) {
	if len(indices) == 0 {
		return
	}
	f.DomainDim -= len(indices) - 1

	slices.Sort(indices)
	min := indices[0]

	replacements := map[int]Expression{}
	for i := 1; i < len(indices); i++ {
		replacements[indices[i]] = IdentityExp(min)
	}
	f.exp = f.exp.Replace(replacements)
}

func (f *Function) AddToDomain(n int) {
	f.DomainDim += n
}

func (f *Function) FunctionSubstitution(substIndex int, domIndices []int, g Function) {
	// First update g's expression so that any indices are now with respect to f's domain
	gReplacements := map[int]Expression{}
	for i, index := range domIndices {
		if index == substIndex {
			panic("The index being substituted cannot also be an input to the replacement function")
		}
		if index > substIndex {
			index -= 1
		}
		gReplacements[i] = IdentityExp(index)
	}

	// Remove substIndex from f's domain and substitute in the transformed expression for g
	replacements := map[int]Expression{}
	for i := substIndex + 1; i < f.DomainDim; i++ {
		replacements[i] = IdentityExp(i - 1)
	}
	replacements[substIndex] = g.exp.Replace(gReplacements)
	f.exp = f.exp.Replace(replacements)

	f.DomainDim -= 1
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
		DomainDim:   domainDim,
		CodomainDim: 1,
		exp:         exp,
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

func (b binOp) Replace(replacements map[int]Expression) Expression {
	return binOp{
		e1: b.e1.Replace(replacements),
		e2: b.e2.Replace(replacements),
		op: b.op,
	}
}

func (b binOp) String() string {
	return fmt.Sprintf("(%v) %v (%v)", b.e1, b.op, b.e2)
}

func (n number) Eval(input []int) int {
	return int(n)
}

func (n number) Replace(replacements map[int]Expression) Expression {
	return n
}

func (n number) String() string {
	return strconv.Itoa(int(n))
}

func (i identity) Eval(input []int) int {
	return input[i.index]
}

func (i identity) Replace(replacements map[int]Expression) Expression {
	if exp, ok := replacements[i.index]; ok {
		return exp
	}
	return i
}

func (i identity) String() string {
	return fmt.Sprintf("x.%d", i.index)
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
