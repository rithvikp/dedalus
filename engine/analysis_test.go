package engine

import (
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rithvikp/dedalus/ast"
)

func TestSubComponents(t *testing.T) {
	tests := []struct {
		msg              string
		source           string
		subComponents    [][]string // TODO: Rule id's are used directly in this test, which is a little brittle
		ingressRelations [][]string
		egressRelations  [][]string
	}{
		{
			msg: "One component",
			source: `
out1(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(a,b,c,l,t) :- out1(a,b,c,l,t)`,
			subComponents:    [][]string{{"0", "1"}},
			ingressRelations: [][]string{{"in1", "in2"}, {"out1"}},
			egressRelations:  [][]string{{"out1"}, {"out2"}},
		},
		{
			msg: "Two unrelated sub-components",
			source: `
out1(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(a,b,c,l,t) :- in2(b,c,l,t), in1(a,b,l,t)`,
			subComponents:    [][]string{{"0"}, {"1"}},
			ingressRelations: [][]string{{"in1", "in2"}, {"out1"}},
			egressRelations:  [][]string{{"in1", "in2"}, {"out2"}},
		},
		{
			msg: "Two unrelated sub-components with choose",
			source: `
location("L2").
out1(a,b,c,l',t') :- in1(a,b,l,t), in2(b,c,l,t), choose((a,b,c), t'), location(l')
out2(a,b,c,l,t) :- in2(b,c,l,t), in1(a,b,l,t)`,
			subComponents:    [][]string{{"0"}, {"1"}},
			ingressRelations: [][]string{{"in1", "in2"}, {"out1"}},
			egressRelations:  [][]string{{"in1", "in2"}, {"out2"}},
		},
		{
			msg: "Two 1-rule components",
			source: `
location("L2").
out1(a,b,c,l',t') :- in1(a,b,l,t), in2(b,c,l,t), choose((a,b,c), t'), location(l')
out2(a,b,c,l,t) :- out1(a,b,c,l,t)`,
			subComponents:    [][]string{{"0"}, {"1"}},
			ingressRelations: [][]string{{"in1", "in2"}, {"out1"}},
			egressRelations:  [][]string{{"out1"}, {"out2"}},
		},
		{
			msg: "Two 2-rule components",
			source: `
location("L2").
out1(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(a,b,c,l',t') :- out1(a,b,c,l,t), choose((a,b,c), t'), location(l')
out3(a,b,c,l,t) :- out2(a,b,c,l,t)
out4(a,b,c,l,t) :- out3(a,b,c,l,t)`,
			subComponents:    [][]string{{"0", "1"}, {"2", "3"}},
			ingressRelations: [][]string{{"in1", "in2"}, {"out2"}},
			egressRelations:  [][]string{{"out2"}, {"out4"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			p, err := ast.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Errorf("unable to parse the program: %v", err)
				return
			}

			s, err := New(p)
			if err != nil {
				t.Errorf("unable to initialize the engine state: %v", err)
				return
			}

			subComponents := s.SubComponents()
			if len(tt.subComponents) != len(subComponents) {
				// TODO: Print a diff here as well.
				t.Errorf("got %d components, wanted %d", len(subComponents), len(tt.subComponents))
				return
			}

			// TODO: Fix order dependence
			for i, got := range subComponents {
				want := tt.subComponents[i]

				ids := make([]string, 0, len(got.Rules))
				for _, rl := range got.Rules {
					ids = append(ids, rl.id)
				}
				sort.Strings(ids)
				sort.Strings(want)

				if diff := cmp.Diff(ids, want); diff != "" {
					t.Errorf("rules in component diff (-got, +want):\n%s", diff)
				}
			}
		})
	}
}
