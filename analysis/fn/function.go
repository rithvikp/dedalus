package fn

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
)

// TODO: Some of this functionality is duplicated by the runtime's internal expression system.
type Func struct {
	DomainDim   int
	CodomainDim int
	exp         Expression
}

type Expression interface {
	Eval(input []int) int
	replace(replacements map[int]Expression) Expression
	String() string
}

func (f Func) String() string {
	return fmt.Sprintf("{ Dom: %d, Codom: %d, Exp: %v }", f.DomainDim, f.CodomainDim, f.exp)
}

func (f *Func) Clone() Func {
	g := *f

	return g
}

func (f *Func) Eval(x []int) int {
	return f.exp.Eval(x)
}

func (f *Func) MergeDomain(indices []int) {
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
	f.exp = f.exp.replace(replacements)
}

func (f *Func) AddToDomain(n int) {
	f.DomainDim += n
}

func (f *Func) SubstituteFunc(substIndex int, domIndices []int, g Func) {
	// First update g's expression so that any indices are now with respect to f's domain
	greplacements := map[int]Expression{}
	for i, index := range domIndices {
		if index == substIndex {
			panic("The index being substituted cannot also be an input to the replacement function")
		}
		if index > substIndex {
			index -= 1
		}
		greplacements[i] = IdentityExp(index)
	}

	// Remove substIndex from f's domain and substitute in the transformed expression for g
	replacements := map[int]Expression{}
	for i := substIndex + 1; i < f.DomainDim; i++ {
		replacements[i] = IdentityExp(i - 1)
	}
	replacements[substIndex] = g.exp.replace(greplacements)
	f.exp = f.exp.replace(replacements)

	f.DomainDim -= 1
}

func (f *Func) DangerouslyReplaceExp(replacements map[int]Expression) {
	f.exp = f.exp.replace(replacements)
}

func (f *Func) Exp() Expression {
	return f.exp
}

func Identity() Func {
	return FromExpr(IdentityExp(0), 1)
}

func Const(val int) Func {
	return FromExpr(Number(val), 0)
}

func BlackBox(id string, domainDim int, metadata any) Func {
	indices := make([]int, domainDim)
	for i := 0; i < domainDim; i++ {
		indices[i] = i
	}
	return FromExpr(BlackBoxExp(id, indices, metadata), domainDim)
}

func NestedBlackBox(id string, domainDim, blackBoxDomainDim int, transformations map[int]Expression, metadata any) Func {
	inputs := make([]Expression, blackBoxDomainDim)
	for i := 0; i < blackBoxDomainDim; i++ {
		if exp, ok := transformations[i]; ok {
			inputs[i] = exp
		} else {
			inputs[i] = IdentityExp(i)
		}
	}
	return FromExpr(BlackBoxExpWithInputs(id, inputs, metadata), domainDim)
}

func FromExpr(exp Expression, domainDim int) Func {
	return Func{
		DomainDim:   domainDim,
		CodomainDim: 1,
		exp:         exp,
	}
}

func Equal(a, b Func) bool {
	if a.DomainDim != b.DomainDim {
		return false
	} else if a.CodomainDim != b.CodomainDim {
		return false
	}

	return exprEqual(a.exp, b.exp, a.DomainDim)
}

func exprEqual(a, b Expression, domainDim int) bool {
	blackBox1, blackBox1Ok := a.(blackBox)
	blackBox2, blackBox2Ok := b.(blackBox)

	if blackBox1Ok && blackBox2Ok {
		if blackBox1.id != blackBox2.id {
			return false
		} else if len(blackBox1.inputs) != len(blackBox2.inputs) {
			return false
		}
		for i := 0; i < len(blackBox1.inputs); i++ {
			if !exprEqual(blackBox1.inputs[i], blackBox2.inputs[i], domainDim) {
				return false
			}
		}
		return true
	} else if blackBox1Ok || blackBox2Ok {
		panic("Cannot compare a black box expression to one that is not also black box")
	}

	// This is a very basic heuristic for "equality" of expressions
	values := []int{0, 1, 31, 100}
	for _, v := range values {
		input := make([]int, domainDim)
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

type blackBox struct {
	id       string
	metadata any
	inputs   []Expression
}

func (b blackBox) replace(replacements map[int]Expression) Expression {
	newInputs := make([]Expression, len(b.inputs))
	for i, input := range b.inputs {
		newInputs[i] = input.replace(replacements)
	}
	return blackBox{id: b.id, metadata: b.metadata, inputs: newInputs}
}

func (b blackBox) String() string {
	bs := strings.Builder{}
	bs.WriteString(fmt.Sprintf("BlackBox(%s {", b.id))

	for i, input := range b.inputs {
		bs.WriteString(fmt.Sprintf("%d: %v", i, input))
		if i < len(b.inputs)-1 {
			bs.WriteString(", ")
		}
	}

	bs.WriteString("})")
	return bs.String()
}

func (b blackBox) Eval(input []int) int {
	panic("Eval is not implemented for black box expressions")
}

//func traverseBlackBoxExp(exp Expression) {
//b, ok := exp.(blackBox)
//if !ok {
//panic("The provided expression was not a black-box expression.")
//}

//rel := b.metadata.(*engine.Relation)

//inputs := make([]IndexOrAttr, len(b.inputs))
//for i, input := range b.inputs {
//if ident, ok := input.(identity); ok {
//index := ident.index
//inputs[i] = IndexOrAttr{Index: &index}
//} else {
//attr := traverseBlackBoxExp(input, visit)
//inputs[i] = IndexOrAttr{Attr: &attr}
//}
//}
//return visit(rel, inputs)
//}

type binOp struct {
	e1 Expression
	e2 Expression
	op string
}

type Number int

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

func (b binOp) replace(replacements map[int]Expression) Expression {
	return binOp{
		e1: b.e1.replace(replacements),
		e2: b.e2.replace(replacements),
		op: b.op,
	}
}

func (b binOp) String() string {
	return fmt.Sprintf("(%v) %v (%v)", b.e1, b.op, b.e2)
}

func (n Number) Eval(input []int) int {
	return int(n)
}

func (n Number) replace(replacements map[int]Expression) Expression {
	return n
}

func (n Number) String() string {
	return strconv.Itoa(int(n))
}

func (i identity) Eval(input []int) int {
	return input[i.index]
}

func (i identity) replace(replacements map[int]Expression) Expression {
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

func BlackBoxExp(id string, indices []int, metadata any) Expression {
	b := blackBox{
		id:       id,
		metadata: metadata,
		inputs:   make([]Expression, len(indices)),
	}
	for i, inputIndex := range indices {
		b.inputs[i] = IdentityExp(inputIndex)
	}
	return b
}

func BlackBoxExpWithInputs(id string, inputs []Expression, metadata any) Expression {
	b := blackBox{
		id:       id,
		metadata: metadata,
		inputs:   inputs,
	}
	return b
}

// TODO: These internals methods are sub-optimal. Investigate improving this abstraction.
type IndexOrExp struct {
	Index *int
	Exp   Expression
}

// BlackBoxInternals returns the metadata and inputs for the provided black box expression.
func BlackBoxInternals(exp Expression) ([]IndexOrExp, any, bool) {
	b, ok := exp.(blackBox)
	if !ok {
		return nil, nil, false
	}

	inputs := make([]IndexOrExp, len(b.inputs))
	for i, input := range b.inputs {
		if ident, ok := input.(identity); ok {
			index := ident.index
			inputs[i] = IndexOrExp{Index: &index}
		} else {
			inputs[i] = IndexOrExp{Exp: input}
		}
	}
	return inputs, b.metadata, true
}

func IdentityInternals(exp Expression) (int, bool) {
	ident, ok := exp.(identity)
	return ident.index, ok
}
