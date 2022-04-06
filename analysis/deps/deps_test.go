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
	fds := DepClosure(s.Rules()[0], existingFDs, false)
	for _, dep := range fds.Elems() {
		fmt.Printf("\n%v\n\n", dep)
	}
	fmt.Println("Num FDs:", len(fds.Elems()))
}

func TestDeps(t *testing.T) {
	tests := []struct {
		msg     string
		program string
		vFDs    func(*engine.Rule) *SetFunc[*varFD]
	}{
		{
			msg:     "Reflexive FDs",
			program: `out(a,b,c,l,t) :- in1(a,b,c,l,t)`,
			vFDs: func(rl *engine.Rule) *SetFunc[*varFD] {
				var vFDs []*varFD
				for _, v := range rl.HeadVars() {
					vFDs = append(vFDs, &varFD{
						Dom:   []*engine.Variable{v},
						Codom: v,
						f:     IdentityFunc(),
					})
				}

				s := &SetFunc[*varFD]{equal: varFDEqual}
				s.Add(vFDs...)
				return s
			},
		},
		{
			msg: "Operation FD",
			program: `add("1", "2", "3").
out(a,c,l,t) :- in1(a,l,t), add(a,1,c)`,
			vFDs: func(rl *engine.Rule) *SetFunc[*varFD] {
				var vFDs []*varFD
				for _, v := range rl.HeadVars() {
					vFDs = append(vFDs, &varFD{
						Dom:   []*engine.Variable{v},
						Codom: v,
						f:     IdentityFunc(),
					})
				}

				vFDs = append(vFDs, &varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[0]},
					Codom: rl.HeadVars()[1],
					f:     ExprFunc(AddExp(IdentityExp(0), number(1)), 1),
				})

				s := &SetFunc[*varFD]{equal: varFDEqual}
				s.Add(vFDs...)
				return s
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			existingFDs := map[*engine.Relation]*SetFunc[*FD]{}
			s := stateFromProgram(t, tt.program)
			rl := s.Rules()[0]
			got := Deps(rl, existingFDs, false)
			want := tt.vFDs(rl)

			if !got.Equal(want) {
				t.Errorf("fds from Dep not equal: got %+v, \n\n want %+v", got, want)
			}
		})
	}
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
	y := &varFD{
		Dom:   []*engine.Variable{c},
		Codom: d,
		f:     ExprFunc(AddExp(IdentityExp(0), number(2)), 1),
	}
	z := &varFD{
		Dom:   []*engine.Variable{d},
		Codom: e,
		f:     ExprFunc(AddExp(IdentityExp(0), number(3)), 1),
	}

	tests := []struct {
		msg            string
		transformation func() *varFD
		output         *varFD
	}{
		{
			msg:            "Single substitution: h(a,b,c) --> h(a, g(a), c)",
			transformation: func() *varFD { return funcSub(g, h) },
			output: &varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: d,
				f:     ExprFunc(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)), 2),
			},
		},
		{
			msg:            "Single substitution (multi-var subst. domain): f(a,b,c,d) --> f(a,b,c,h(a,b,c))",
			transformation: func() *varFD { return funcSub(h, f) },
			output: &varFD{
				Dom:   []*engine.Variable{a, b, c},
				Codom: e,
				f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2))), 3),
			},
		},
		{
			msg:            "Two substitutions: f(a,b,c,d) --> f(a, g(a), c, y(c)",
			transformation: func() *varFD { return funcSub(y, funcSub(g, f)) },
			output: &varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)), AddExp(IdentityExp(1), number(2))), 2),
			},
		},
		{
			msg:            "Domain-increasing substitution: z(d) --> z(h(a,b,c))",
			transformation: func() *varFD { return funcSub(h, z) },
			output: &varFD{
				Dom:   []*engine.Variable{a, b, c},
				Codom: e,
				f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(2)), number(3)), 3),
			},
		},
		{
			msg:            "Nested transformation: f(a,b,c,d) --> f(a, g(a), c, h(a,g(a),c))",
			transformation: func() *varFD { return funcSub(g, funcSub(h, f)) },
			output: &varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f: ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)),
					AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1))), 2),
			},
		},
		{
			// Requires updating all future substitutions with any relevant previous substitutions
			// (substitute g(a) into h(a,b,c) before substituting h into f).
			msg:            "Nested transformation (other direction): f(a,b,c,d) --> f(a, g(a), c, h(a,g(a),c))",
			transformation: func() *varFD { return funcSub(h, funcSub(g, f)) },
			output: &varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f: ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)),
					AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1))), 2),
			},
		},
		{
			msg:            "Domain-increasing nested transformation: z(d) --> z(h(a,g(a),c))",
			transformation: func() *varFD { return funcSub(g, funcSub(h, z)) },
			output: &varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f:     ExprFunc(AddExp(AddExp(AddExp(IdentityExp(0), AddExp(IdentityExp(0), number(3))), IdentityExp(1)), number(3)), 2),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			got := tt.transformation()
			if !varFDEqual(got, tt.output) {
				t.Errorf("fds not equal: got %+v, \n\n want %+v", got, tt.output)
			}
		})
	}
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
