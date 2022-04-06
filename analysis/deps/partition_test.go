package deps

import (
	"testing"

	"github.com/rithvikp/dedalus/engine"
)

// TODO: This test function currently just outputs to stdout for manual inspection. This will be changed soon.
func TestDistributionPolicy(t *testing.T) {
	tests := []struct {
		msg      string
		program  string
		policies func(s *engine.State) []DistPolicy
	}{
		{
			msg:     "Join with add",
			program: `out(a,b,c,l,t) :- in1(a,b,c,l,t), add(a,b,c)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]

				var policies []DistPolicy
				for i := 0; i < 3; i++ {
					policies = append(policies, DistPolicy{
						in1: DistFunction{Dom: in1.Attrs()[i : i+1], f: IdentityFunc()},
					})
				}
				return policies
			},
		},
		{
			msg:     "Two relation join with add",
			program: `out(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				in1 := s.Rules()[0].Body()[0]
				in2 := s.Rules()[0].Body()[2]

				var policies []DistPolicy
				policies = append(policies, DistPolicy{
					in1: DistFunction{Dom: in1.Attrs()[0:1], f: ExprFunc(AddExp(IdentityExp(0), number(1)), 1)},
					in2: DistFunction{Dom: in2.Attrs()[0:1], f: IdentityFunc()},
				})
				return policies
			},
		},
		{
			msg: "2 rule, unpartitionable program",
			program: `
				out1(a,c,l,t) :- in1(a,l,t), add(a,1,c), in2(c,l,t)
				out2(a,c,l,t) :- in1(a,l,t), add(a,2,c), in2(c,l,t)`,
			policies: func(s *engine.State) []DistPolicy {
				var policies []DistPolicy
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

	//s := stateFromProgram(t, preface+"\n"+tests[2].program)
	//policies := DistibutionPolicies(s)
	//for _, p := range policies {
	//fmt.Print("\n============================\n")
	//for rel, df := range p {
	//fmt.Printf("\n%v -> %v\n\n", rel.ID(), df)
	//}
	//fmt.Printf("\n============================\n")
	//}
}
