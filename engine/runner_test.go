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
			msg: "join, same time",
			source: `
out1(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(a,b,c,l,t) :- in2(b,c,l,t), in1(a,b,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
				"out2": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "large join, same time",
			source: `
out(a,b,c,d,f,l,t) :- in1(a,b,l,t), in2(b,c,l,t), in3(c,d,l,t), in4(e,f,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).
in3("3","4",L1,0).
in3("4","5",L1,0).
in4("1","2",L1,0).
in4("3","4",L1,0).`,
			facts: map[string][]*fact{
				"out": {
					{[]string{"1", "2", "3", "4", "2"}, "L1", 0},
					{[]string{"1", "2", "3", "4", "4"}, "L1", 0},
					{[]string{"3", "2", "3", "4", "2"}, "L1", 0},
					{[]string{"3", "2", "3", "4", "4"}, "L1", 0},
				},
			},
		},
		{
			msg: "join, successor",
			source: `
out1(a,b,c,l,t') :- in1(a,b,l,t), in2(b,c,l,t), succ(t,t')
out2(a,b,c,l,t') :- in2(b,c,l,t), in1(a,b,l,t), succ(t,t')
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"1", "2", "3"}, "L1", 1}, {[]string{"3", "2", "3"}, "L1", 1}},
				"out2": {{[]string{"1", "2", "3"}, "L1", 1}, {[]string{"3", "2", "3"}, "L1", 1}},
			},
		},
		{
			msg: "self-join, same time",
			source: `
out(a,l,t) :- in1(a,a,l,t)
in1("a","a",L1,0).
in1("a","b",L1,0).`,
			facts: map[string][]*fact{
				"out": {
					{[]string{"a"}, "L1", 0},
				},
			},
		},
		{
			msg: "self-join with multiple relations, same time",
			source: `
out1(a,b,l,t) :- in1(a,a,l,t), in2(a,b,l,t)
out2(a,b,l,t) :- in2(a,b,l,t), in1(a,a,l,t)
in1("a","a",L1,0).
in1("a","b",L1,0).
in1("b","c",L1,0).
in2("a","b",L1,0).
in2("a","c",L1,0).
in2("b","c",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"a", "b"}, "L1", 0}, {[]string{"a", "c"}, "L1", 0}},
				"out2": {{[]string{"a", "b"}, "L1", 0}, {[]string{"a", "c"}, "L1", 0}},
			},
		},
		{
			msg: "self-join with multiple relations but no other joins, same time",
			source: `
out1(a,b,l,t) :- in1(a,a,l,t), in2(b,c,l,t)
out2(a,b,l,t) :- in2(b,c,l,t), in1(a,a,l,t)
in1("a","a",L1,0).
in1("a","b",L1,0).
in1("b","c",L1,0).
in2("a","b",L1,0).
in2("b","c",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"a", "a"}, "L1", 0}, {[]string{"a", "b"}, "L1", 0}},
				"out2": {{[]string{"a", "a"}, "L1", 0}, {[]string{"a", "b"}, "L1", 0}},
			},
		},
		{
			msg: "join on loc and time, same time",
			source: `
out1(a,b,l,t) :- in1(a,t,l,t), in2(b,l,l,t)
out2(a,b,l,t) :- in2(b,l,l,t), in1(a,t,l,t)
in1("1","0",L1,0).
in1("2","1",L1,0).
in2("3","L1",L1,0).
in2("4","L2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"1", "3"}, "L1", 0}},
				"out2": {{[]string{"1", "3"}, "L1", 0}},
			},
		},
		{
			msg: "max aggregation, same time",
			source: `
out1(max<a>,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(max<a>,b,c,l,t) :- in2(b,c,l,t), in1(a,b,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"3", "2", "3"}, "L1", 0}},
				"out2": {{[]string{"3", "2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "min aggregation, same time",
			source: `
out(min<a>,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"1", "2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "sum aggregation, same time",
			source: `
out(b,sum<a>,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"2", "4", "3"}, "L1", 0}},
			},
		},
		{
			msg: "count aggregation, same time",
			source: `
out(b,count<a>,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"2", "2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "negation, same time",
			source: `
out1(a,b,l,t) :- in1(a,b,l,t), not in2(a,b,l,t)
out2(a,b,l,t) :- not in2(a,b,l,t), in1(a,b,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"3", "2"}, "L1", 0}},
				"out2": {{[]string{"3", "2"}, "L1", 0}},
			},
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

			r, err := NewRunner(p)
			if err != nil {
				t.Errorf("unable to initialize the runner: %v", err)
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
