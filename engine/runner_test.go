package engine

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rithvikp/dedalus/ast"
)

func lessFacts(a, b *fact) bool {
	if a.timestamp < b.timestamp {
		return true
	} else if a.timestamp > b.timestamp {
		return false
	}

	if a.location < b.location {
		return true
	} else if a.location > b.location {
		return false
	}

	// If the location and timestamp are the same, sort by fact data
	for i := 0; i < len(a.data) && i < len(b.data); i++ {
		if a.data[i] < b.data[i] {
			return true
		} else if a.data[i] > b.data[i] {
			return false
		}
	}

	return len(a.data) <= len(b.data)
}

func TestExecution(t *testing.T) {
	tests := []struct {
		msg    string
		source string
		facts  map[string][]*fact
	}{
		{
			msg: "basic join, same time",
			source: `
out(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {
					{[]string{"1", "2", "3"}, "L1", 0},
					{[]string{"3", "2", "3"}, "L1", 0},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			p, err := ast.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Errorf("Unable to parse the program: %v\n", err)
				return
			}

			r, err := NewRunner(p)
			if err != nil {
				t.Errorf("Unable to initialize the runner: %v\n", err)
				return
			}

			r.Step()

			for rel, want := range tt.facts {
				got := r.relations[rel].allAcrossSpaceTime()

				if diff := cmp.Diff(got, want, cmp.AllowUnexported(fact{}), cmpopts.SortSlices(lessFacts)); diff != "" {
					t.Errorf("fact diff (-got, +want):\n%s", diff)
				}
			}
		})
	}
}
