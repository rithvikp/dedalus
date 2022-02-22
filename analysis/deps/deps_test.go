package deps

import (
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

func TestDep(t *testing.T) {

}

func TestFuncSub(t *testing.T) {
	// Single substitution
	a := &engine.Variable{}
	b := &engine.Variable{}
	c := &engine.Variable{}
	d := &engine.Variable{}
	e := &engine.Variable{}

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
		f: ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)),
			AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2))), 3),
	}
	got = funcSub(h, f)
	if !varFDEqual(got, want) {
		t.Errorf("fds not equal for funcSub(h,f): got %+v, \n\n want %+v", got, want)
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
