package deps

import (
	"strings"
	"testing"

	"github.com/rithvikp/dedalus/analysis/fn"
	"github.com/rithvikp/dedalus/ast"
	"github.com/rithvikp/dedalus/engine"
)

const preface = `add("1", "2", "3").
f("a","b","c").
g("a","b","c").
`

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

func TestFDs(t *testing.T) {
	tests := []struct {
		msg     string
		program string
		fds     func(*engine.State) map[*engine.Relation]*SetFunc[FD]
	}{
		{
			msg: "Black Box FD",
			program: `out(a,b,c,l,t) :- in1(a,b,l,t), f(a,b,c)
					  out(a,b,c,l,t) :- in2(a,b,l,t), f(a,b,c)`,
			fds: func(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
				fds := map[*engine.Relation]*SetFunc[FD]{}
				rl := s.Rules()[0]
				fds[rl.Head()] = &SetFunc[FD]{equal: fdEqual}
				fds[rl.Head()].Add(FD{
					Dom:   []engine.Attribute{rl.Head().Attrs()[0], rl.Head().Attrs()[1]},
					Codom: rl.Head().Attrs()[2],
					f:     fn.BlackBox("f", 2, nil),
				})

				return fds
			},
		},
		{
			msg:     "Multiple Black Box FDs",
			program: `out(a,b,c,d,e,f,l,t) :- in1(a,b,d,e,l,t), f(a,b,c), g(d,e,f)`,
			fds: func(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
				fds := map[*engine.Relation]*SetFunc[FD]{}
				rl := s.Rules()[0]
				fds[rl.Head()] = &SetFunc[FD]{equal: fdEqual}
				fds[rl.Head()].Add(FD{
					Dom:   []engine.Attribute{rl.Head().Attrs()[0], rl.Head().Attrs()[1]},
					Codom: rl.Head().Attrs()[2],
					f:     fn.BlackBox("f", 2, nil),
				})
				fds[rl.Head()].Add(FD{
					Dom:   []engine.Attribute{rl.Head().Attrs()[3], rl.Head().Attrs()[4]},
					Codom: rl.Head().Attrs()[5],
					f:     fn.BlackBox("g", 2, nil),
				})

				return fds
			},
		},
		{
			msg:     "Black Box FD where one var only appears in the contraint (in the body)",
			program: `out(a,b,c,l,t) :- in1(a,l,t), f(a,b,c), in2(c,l,t)`,
			fds: func(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
				fds := map[*engine.Relation]*SetFunc[FD]{}
				rl := s.Rules()[0]
				fds[rl.Head()] = &SetFunc[FD]{equal: fdEqual}
				fds[rl.Head()].Add(FD{
					Dom:   []engine.Attribute{rl.Head().Attrs()[0], rl.Head().Attrs()[1]},
					Codom: rl.Head().Attrs()[2],
					f:     fn.BlackBox("f", 2, nil),
				})

				return fds
			},
		},
		{
			msg:     "No FDs if relevant attributes aren't also in the head",
			program: `out(a,c,l,t) :- in1(a,b,l,t), f(a,b,c)`,
			fds: func(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
				return map[*engine.Relation]*SetFunc[FD]{}
			},
		},
		{
			msg: "No FDs if relationships are not consistent across rules",
			program: `out1(a,b,c,l,t) :- in1(a,b,l,t), f(a,b,c)
					  out1(a,b,c,l,t) :- in2(a,b,l,t), f(b,a,c)

					  out2(a,b,c,l,t) :- in1(a,b,l,t), f(a,b,c)
					  out2(c,a,b,l,t) :- in2(a,b,l,t), f(a,b,c)`,
			fds: func(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
				return map[*engine.Relation]*SetFunc[FD]{}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			s := stateFromProgram(t, preface+"\n"+tt.program)
			got := FDs(s)
			want := tt.fds(s)

			checkMatch := func(rel *engine.Relation, fds *SetFunc[FD], toCheck map[*engine.Relation]*SetFunc[FD]) bool {
				if _, ok := want[rel]; !ok {
					if fds.Len() == 0 {
						return true
					}
					return false
				}
				return fds.Equal(toCheck[rel])
			}
			equal := true
			for rel, fds := range got {
				if !checkMatch(rel, fds, want) {
					equal = false
					break
				}
			}
			for rel, fds := range want {
				if !checkMatch(rel, fds, got) {
					equal = false
					break
				}
			}

			if !equal {
				t.Errorf("Derived fds not equal: got %+v, \n\n want %+v", got, want)
			}
		})
	}
}

func TestDeps(t *testing.T) {
	reflexiveDeps := func(rl *engine.Rule) []varFD {
		var vFDs []varFD
		for _, v := range rl.HeadVars() {
			vFDs = append(vFDs, varFD{
				Dom:   []*engine.Variable{v},
				Codom: v,
				f:     fn.Identity(),
			})
		}
		return vFDs
	}

	setify := func(vFDs []varFD) *SetFunc[varFD] {
		s := &SetFunc[varFD]{equal: varFDEqual}
		s.Add(vFDs...)
		return s
	}

	tests := []struct {
		msg     string
		program string
		vFDs    func(*engine.Rule) *SetFunc[varFD]
	}{
		{
			msg:     "Reflexive FDs",
			program: `out(a,b,c,l,t) :- in1(a,b,c,l,t)`,
			vFDs: func(rl *engine.Rule) *SetFunc[varFD] {
				var vFDs = reflexiveDeps(rl)
				return setify(vFDs)
			},
		},
		{
			msg:     "Operation FD",
			program: `out(a,c,l,t) :- in1(a,l,t), add(a,1,c)`,
			vFDs: func(rl *engine.Rule) *SetFunc[varFD] {
				var vFDs = reflexiveDeps(rl)

				vFDs = append(vFDs, varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[0]},
					Codom: rl.HeadVars()[1],
					f:     fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(1)), 1),
				})

				return setify(vFDs)
			},
		},
		{
			msg:     "Black Box FD",
			program: `out(a,b,c,l,t) :- in1(a,b,l,t), f(a,b,c)`,
			vFDs: func(rl *engine.Rule) *SetFunc[varFD] {
				var vFDs = reflexiveDeps(rl)

				vFDs = append(vFDs, varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[0], rl.HeadVars()[1]},
					Codom: rl.HeadVars()[2],
					f:     fn.BlackBox("f", 2, nil),
				})

				return setify(vFDs)
			},
		},
		{
			msg:     "Chained Black Box FD",
			program: `out(a,b,c,d,e,l,t) :- in1(a,b,d,l,t), f(a,b,c), g(c,d,e)`,
			vFDs: func(rl *engine.Rule) *SetFunc[varFD] {
				var vFDs = reflexiveDeps(rl)

				vFDs = append(vFDs, varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[0], rl.HeadVars()[1]},
					Codom: rl.HeadVars()[2],
					f:     fn.BlackBox("f", 2, nil),
				})
				vFDs = append(vFDs, varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[2], rl.HeadVars()[3]},
					Codom: rl.HeadVars()[4],
					f:     fn.BlackBox("g", 2, nil),
				})

				return setify(vFDs)
			},
		},
		{
			msg:     "Black Box FD where one var only appears in the contraint (in the body)",
			program: `out(a,b,c,l,t) :- in1(a,l,t), f(a,b,c), in2(c,l,t)`,
			vFDs: func(rl *engine.Rule) *SetFunc[varFD] {
				var vFDs = reflexiveDeps(rl)

				vFDs = append(vFDs, varFD{
					Dom:   []*engine.Variable{rl.HeadVars()[0], rl.HeadVars()[1]},
					Codom: rl.HeadVars()[2],
					f:     fn.BlackBox("f", 2, nil),
				})

				return setify(vFDs)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			existingFDs := map[*engine.Relation]*SetFunc[FD]{}
			s := stateFromProgram(t, preface+"\n"+tt.program)
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

	g := varFD{
		Dom:   []*engine.Variable{a},
		Codom: b,
		f:     fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(3)), 1),
	}
	h := varFD{
		Dom:   []*engine.Variable{a, b, c},
		Codom: d,
		f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2)), 3),
	}
	f := varFD{
		Dom:   []*engine.Variable{a, b, c, d},
		Codom: e,
		f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2)), fn.IdentityExp((3))), 4),
	}
	y := varFD{
		Dom:   []*engine.Variable{c},
		Codom: d,
		f:     fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(2)), 1),
	}
	z := varFD{
		Dom:   []*engine.Variable{d},
		Codom: e,
		f:     fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(3)), 1),
	}

	tests := []struct {
		msg            string
		transformation func() varFD
		output         varFD
	}{
		{
			msg:            "Single substitution: h(a,b,c) --> h(a, g(a), c)",
			transformation: func() varFD { return funcSub(g, h) },
			output: varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: d,
				f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1)), 2),
			},
		},
		{
			msg:            "Single substitution (multi-var subst. domain): f(a,b,c,d) --> f(a,b,c,h(a,b,c))",
			transformation: func() varFD { return funcSub(h, f) },
			output: varFD{
				Dom:   []*engine.Variable{a, b, c},
				Codom: e,
				f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2)), fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2))), 3),
			},
		},
		{
			msg:            "Two substitutions: f(a,b,c,d) --> f(a, g(a), c, y(c)",
			transformation: func() varFD { return funcSub(y, funcSub(g, f)) },
			output: varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1)), fn.AddExp(fn.IdentityExp(1), fn.Number(2))), 2),
			},
		},
		{
			msg:            "Domain-increasing substitution: z(d) --> z(h(a,b,c))",
			transformation: func() varFD { return funcSub(h, z) },
			output: varFD{
				Dom:   []*engine.Variable{a, b, c},
				Codom: e,
				f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2)), fn.Number(3)), 3),
			},
		},
		{
			msg:            "Nested transformation: f(a,b,c,d) --> f(a, g(a), c, h(a,g(a),c))",
			transformation: func() varFD { return funcSub(g, funcSub(h, f)) },
			output: varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f: fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1)),
					fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1))), 2),
			},
		},
		{
			// Requires updating all future substitutions with any relevant previous substitutions
			// (substitute g(a) into h(a,b,c) before substituting h into f).
			msg:            "Nested transformation (other direction): f(a,b,c,d) --> f(a, g(a), c, h(a,g(a),c))",
			transformation: func() varFD { return funcSub(h, funcSub(g, f)) },
			output: varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f: fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1)),
					fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1))), 2),
			},
		},
		{
			msg:            "Domain-increasing nested transformation: z(d) --> z(h(a,g(a),c))",
			transformation: func() varFD { return funcSub(g, funcSub(h, z)) },
			output: varFD{
				Dom:   []*engine.Variable{a, c},
				Codom: e,
				f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.AddExp(fn.IdentityExp(0), fn.Number(3))), fn.IdentityExp(1)), fn.Number(3)), 2),
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

	h := varOrAttrFD{
		Dom:   []varOrAttr{a, b, c},
		Codom: d,
		f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.IdentityExp(1)), fn.IdentityExp(2)), 3),
	}

	// h(a,b,c) --> h(a,b,3)
	want := varOrAttrFD{
		Dom:   []varOrAttr{a, c},
		Codom: d,
		f:     fn.FromExpr(fn.AddExp(fn.AddExp(fn.IdentityExp(0), fn.Number(3)), fn.IdentityExp(1)), 2),
	}
	got := constSub(h, 3, *b.Attr)
	if !varOrAttrFDEqual(got, want) {
		t.Errorf("fds not equal for constSub(h,3,b): got %+v, \n\n want %+v", h, want)
	}
}
