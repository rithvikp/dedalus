package deps

import (
	"testing"

	"github.com/rithvikp/dedalus/analysis/fn"
	"github.com/rithvikp/dedalus/engine"
)

func TestDistributionPolicy(t *testing.T) {
	tests := []struct {
		msg      string
		program  string
		policies func(s *engine.State) []DistPolicy
	}{
		{
			msg:     "Black Box: 2 relation join",
			program: `out(a,c,l,t) :- in1(a,b,l,t), f(a,b,c), in2(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[2]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:2], f: fn.BlackBox("f", 2)},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
		{
			msg:     "Black Box: Chained dependencies",
			program: `out(a,e,l,t) :- in1(a,b,d,l,t), f(a,b,c), g(c,d,e), in2(e,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[3]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:3], f: fn.NestedBlackBox("g", 3, 2, map[int]fn.Expression{
						0: fn.BlackBoxExp("f", []int{0, 1}),
						1: fn.IdentityExp(2),
					})},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
		{
			msg: "Black Box: Multiple rules with the same dependency",
			program: `out(a,e,l,t) :- in1(a,b,d,l,t), f(a,b,c), g(c,d,e), in2(e,l,t)
					  out(a,e,l,t) :- in3(a,b,d,l,t), f(a,b,c), g(c,d,e), in4(e,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[3]
				in3 := s.Rules()[1].Body()[0]
				in4 := s.Rules()[1].Body()[3]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:3], f: fn.NestedBlackBox("g", 3, 2, map[int]fn.Expression{
						0: fn.BlackBoxExp("f", []int{0, 1}),
						1: fn.IdentityExp(2),
					})},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.Identity()},
				})
				policies = append(policies, DistPolicy{
					in3: DistFunction{Dom: in3.Attrs()[0:3], f: fn.NestedBlackBox("g", 3, 2, map[int]fn.Expression{
						0: fn.BlackBoxExp("f", []int{0, 1}),
						1: fn.IdentityExp(2),
					})},
					in4: DistFunction{Dom: in4.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
		{
			msg:     "Arithmetic: Join with add",
			program: `out(a,b,c,l,t) :- in1(a,b,c,l,t), add(a,b,c)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]

				var policies []DistPolicy
				for i := 0; i < 3; i++ {
					policies = append(policies, DistPolicy{
						in1: DistFunction{Dom: in1.Attrs()[i : i+1], f: fn.Identity()},
					})
				}
				return policies
			},
		},
		{
			msg:     "Arithmetic: 2 relation join with add",
			program: `out(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[2]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(1)), 1)},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
		{
			msg: "Arithmetic: 2 rules with different additive constants",
			program: `
				out1(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)
				out2(a,c,l,t) :- in1(a,l,t), add(a,2,c), in2(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				var policies []DistPolicy
				return policies
			},
		},
		{
			msg: "Arithmetic: 3 rules with chained dependencies",
			program: `out1(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)
					  out2(a,c,l,t) :- in2(a,l,t), add(a,2,c), in3(c,l,t)
					  out3(a,c,l,t) :- in3(a,l,t), add(a,3,c), in4(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[2]
				in3 := s.Rules()[2].Body()[0]
				in4 := s.Rules()[2].Body()[2]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(6)), 1)},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(5)), 1)},
					in3: DistFunction{Dom: in3.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(3)), 1)},
					in4: DistFunction{Dom: in4.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
		{
			msg: "Arithmetic: 2 pairs of 2 rules with chained dependencies",
			program: `out1(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)
					  out2(a,c,l,t) :- in2(a,l,t), add(a,2,c), in3(c,l,t)
					  out3(a,c,l,t) :- in4(a,l,t), add(a,3,c), in5(c,l,t)
					  out4(a,c,l,t) :- in5(a,l,t), add(a,4,c), in6(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[2]
				in3 := s.Rules()[1].Body()[2]

				in4 := s.Rules()[2].Body()[0]
				in5 := s.Rules()[2].Body()[2]
				in6 := s.Rules()[3].Body()[2]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(3)), 1)},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(2)), 1)},
					in3: DistFunction{Dom: in3.Attrs()[0:1], f: fn.Identity()},
				})

				policies = append(policies, DistPolicy{
					in4: DistFunction{Dom: in4.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(7)), 1)},
					in5: DistFunction{Dom: in5.Attrs()[0:1], f: fn.FromExpr(fn.AddExp(fn.IdentityExp(0), fn.Number(4)), 1)},
					in6: DistFunction{Dom: in6.Attrs()[0:1], f: fn.Identity()},
				})
				return policies
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			s := stateFromProgram(t, preface+"\n"+tt.program)
			got := DistibutionPolicies(s)
			want := tt.policies(s)

			gotSet := &SetFunc[DistPolicy]{equal: DistPolicyEqual}
			gotSet.Add(got...)
			wantSet := &SetFunc[DistPolicy]{equal: DistPolicyEqual}
			wantSet.Add(want...)

			if !gotSet.Equal(wantSet) {
				t.Errorf("policies from DistributionPolicy not equal: got %+v \n\n want %+v", got, want)
			}
		})
	}

	//p := `
	//as2(x,z,l',t) :- as(x,y,l,t), f(y,a,l,t), locs(l,l',a)
	//match(x,z,l,t) :- as(x,y,l,t), f(y,a,l,t), bs(a,z,l,t)
	//`
	//s := stateFromProgram(t, preface+"\n"+p)
	//policies := DistibutionPolicies(s)
	//for _, p := range policies {
	//fmt.Print("\n============================\n")
	//for rel, df := range p {
	//fmt.Printf("\n%v -> %v\n\n", rel.ID(), df)
	//}
	//fmt.Printf("\n============================\n")
	//}
}
