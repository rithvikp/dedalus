package deps

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rithvikp/dedalus/ast"
	"github.com/rithvikp/dedalus/engine"
)

func stateFromProgram(t *testing.T, program string) *engine.State {
	t.Helper()
	p, err := ast.Parse(strings.NewReader(program))
	if err != nil {
		t.Fatalf("unable to parse the program: %v", err)
	}

	s, err := engine.New(p)
	if err != nil {
		t.Fatalf("unable to initialize the engine state: %v", err)
	}

	return s
}

var p1 = `add("1", "2", "3").
out(a,b,c,l,t) :- in1(a,b,c,l,t)
`
var p2 = `add("1", "2", "3").
out(a,b,c,l,t) :- in1(a,b,c,l,t), add(a,b,c)
`
var p3 = `add("1", "2", "3").
out(a,b,c,c,l,t) :- in1(a,b,c,l,t), add(a,b,c)
`
var p4 = `add("1", "2", "3").
out(a,b,c,c,l,t) :- in1(a,b,c,l,t), add(a,b,c)
out(a,b,c,d,l,t) :- in2(a,b,c,d,l,t), add(a,b,c)
`

var p5 = `add("1", "2", "3").
out(a,c,l,t) :- in1(a,l,t), add(a,1,c)
`
var p = p5

// TODO: These test functions currently just output to stdout for manual inspection. This will be changed soon.
func TestFDs(t *testing.T) {
	s := stateFromProgram(t, p)

	fds := FDs(s)
	for rel, deps := range fds {
		fmt.Printf("\n==============%s==============\n", rel.ID())
		for _, dep := range deps.Elems() {
			fmt.Printf("\n%v\n\n", dep)
		}
		fmt.Println("Num FDs:", len(deps.Elems()))
		fmt.Printf("\n============================\n")
	}
}

func TestHeadFDs(t *testing.T) {
	s := stateFromProgram(t, p)

	existingFDs := map[*engine.Relation]*SetFunc[*FD]{}
	fds := HeadFDs(s.Rules()[0], existingFDs)
	for _, dep := range fds.Elems() {
		fmt.Printf("\n%v\n\n", dep)
	}
	fmt.Println("Num FDs:", len(fds.Elems()))
}

func TestDepClosure(t *testing.T) {
	s := stateFromProgram(t, p)

	existingFDs := map[*engine.Relation]*SetFunc[*FD]{}
	fds := DepClosure(s.Rules()[0], existingFDs)
	for _, dep := range fds.Elems() {
		fmt.Printf("\n%v\n\n", dep)
	}
	fmt.Println("Num FDs:", len(fds.Elems()))
}

func TestDep(t *testing.T) {
	s := stateFromProgram(t, p)

	existingFDs := map[*engine.Relation]*SetFunc[*FD]{}
	vFDs := Dep(s.Rules()[0], existingFDs, false)
	for _, dep := range vFDs.Elems() {
		fmt.Printf("\n%v\n\n", dep)
	}
	fmt.Println("Num FDs:", len(vFDs.Elems()))
}

func TestFuncSub(t *testing.T) {
	// Use a program to bootstrap data from which a fake FD can be constructed.
	s := stateFromProgram(t, "out(a,b,c,d,e,l,t) :- in(a,b,c,d,e,l,t)")
	vars := s.Rules()[0].HeadVars()

	a := vars[0]
	b := vars[1]
	c := vars[2]
	d := vars[3]
	e := vars[4]

	g := &varFD{
		Dom:   []*engine.Variable{a},
		Codom: b,
		f:     ExprFunc(AddExp(IdentityExp(0), number(3)), 1),
	}
	h := &varFD{
		Dom:   []*engine.Variable{a, b, c},
		Codom: d,
		f:     ExprFunc(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), 3),
	}
	f := &varFD{
		Dom:   []*engine.Variable{a, b, c, d},
		Codom: e,
		f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), IdentityExp((3))), 4),
	}
	z := &varFD{
		Dom:   []*engine.Variable{d},
		Codom: e,
		f:     ExprFunc(AddExp(IdentityExp(0), number(3)), 1),
	}

	// h(a,b,c) --> h(a,g(a),c)
	want := &varFD{
		Dom:   []*engine.Variable{a, c},
		Codom: d,
		f:     ExprFunc(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)), 2),
	}
	got := funcSub(g, h)
	if !varFDEqual(got, want) {
		t.Errorf("fds not equal for funcSub(g,h): got %+v, \n\n want %+v", got, want)
	}

	// f(a,b,c,d) --> f(a,b,c,h(a,b,c))
	want = &varFD{
		Dom:   []*engine.Variable{a, b, c},
		Codom: e,
		f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2))), 3),
	}
	got = funcSub(h, f)
	if !varFDEqual(got, want) {
		t.Errorf("fds not equal for funcSub(h,f): got %+v, \n\n want %+v", got, want)
	}

	// z(d) --> z(h(a,b,c))
	want = &varFD{
		Dom:   []*engine.Variable{a, b, c},
		Codom: e,
		f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), number(3)), 3),
	}
	got = funcSub(h, z)
	if !varFDEqual(got, want) {
		t.Errorf("fds not equal for funcSub(z,h): got %+v, \n\n want %+v", got, want)
	}

	// f(a,b,c,d) --> f(a,g(a),c,h(a,g(a),c))
	// want = &varFD{
	// 	Dom:   []*engine.Variable{a, c},
	// 	Codom: e,
	// 	f: ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)),
	// 		AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1))), 2),
	// }
	// got = funcSub(g, f)
	// got = funcSub(h, got)
	// if !varFDEqual(got, want) {
	// 	t.Errorf("fds not equal for funcSub(g,f)+funcSub(h,f): got %+v, \n\n want %+v", got, want)
	// }
}

func TestConstSub(t *testing.T) {
	// Use a program to bootstrap data from which a fake FD can be constructed.
	s := stateFromProgram(t, "out(a,b,c,d,l,t) :- in(a,b,c,d,l,t)")

	attrs := s.Rules()[0].Head().Attrs()
	a := varOrAttr{Attr: &attrs[0]}
	b := varOrAttr{Attr: &attrs[1]}
	c := varOrAttr{Attr: &attrs[2]}
	d := varOrAttr{Attr: &attrs[3]}

	h := &varOrAttrFD{
		Dom:   []varOrAttr{a, b, c},
		Codom: d,
		f:     ExprFunc(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), 3),
	}

	// h(a,b,c) --> h(a,b,3)
	want := &varOrAttrFD{
		Dom:   []varOrAttr{a, c},
		Codom: d,
		f:     ExprFunc(AddExp(AddExp(IdentityExp(0), number(3)), IdentityExp(1)), 2),
	}
	constSub(h, 3, *b.Attr)
	if !varOrAttrFDEqual(h, want) {
		t.Errorf("fds not equal for constSub(h,3,b): got %+v, \n\n want %+v", h, want)
	}
}
