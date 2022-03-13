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
			msg: "join",
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
			msg: "large join",
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
			msg: "self-join",
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
			msg: "self-join with multiple relations",
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
			msg: "self-join with multiple relations but no other joins",
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
			msg: "join on loc and time",
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
			msg: "underscore identifiers",
			source: `
out(a,b,c,l,t) :- in1(a,b,_,_,l,t), in2(b,_,c,l,t)
in1("1","2","7","9",L1,0).
in1("3","2","8","8",L1,0).
in2("2","3","3",L1,0).
in2("2","5","6",L1,0).
in2("1","2","4",L1,0).`,
			facts: map[string][]*fact{
				"out": {
					{[]string{"1", "2", "3"}, "L1", 0},
					{[]string{"3", "2", "3"}, "L1", 0},
					{[]string{"1", "2", "6"}, "L1", 0},
					{[]string{"3", "2", "6"}, "L1", 0},
				},
			},
		},
		{
			msg: "max aggregation",
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
			msg: "min aggregation",
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
			msg: "sum aggregation",
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
			msg: "count aggregation",
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
			msg: "negation",
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
		{
			msg: "(in)equality condition",
			source: `
out1(a,b,d,l,t) :- in1(a,b,l,t), in2(c,d,l,t), b=c
out2(a,b,d,l,t) :- in2(c,d,l,t), in1(a,b,l,t), b=c
out3(a,b,c,l,t) :- in2(c,d,l,t), in1(a,b,l,t), b!=c
out4(a,b,c,l,t) :- in2(c,d,l,t), in1(a,b,l,t), b!=c
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out1": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
				"out2": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
				"out3": {{[]string{"1", "2", "1"}, "L1", 0}, {[]string{"3", "2", "1"}, "L1", 0}},
				"out4": {{[]string{"1", "2", "1"}, "L1", 0}, {[]string{"3", "2", "1"}, "L1", 0}},
			},
		},
		{
			msg: "equality condition with expressions",
			source: `
out(a,b,c,l,t) :- in1(a,b,l,t), in2(c,d,l,t), c=b-1
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"1", "2", "1"}, "L1", 0}, {[]string{"3", "2", "1"}, "L1", 0}},
			},
		},
		{
			msg: "direct assignments",
			source: `
out(a,b,c,e,l,t) :- in1(a,b,l,t), in2(c,d,l,t), e=c+2, c=b-1
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"1", "2", "1", "3"}, "L1", 0}, {[]string{"3", "2", "1", "3"}, "L1", 0}},
			},
		},
		{
			msg: "user-defined read-only replicated tables",
			source: `
out1(a,b,c,l,t) :- in1(a,b), in2(b,c,l,t)
out2(a,b,c,l,t) :- in2(b,c,l,t), in1(a,b)
out3(b,c,l,t) :- in2(b,c,l,t), in3(c)
in1("1","2").
in1("3","2").
in2("2","3",L1,0).
in2("1","2",L1,0).
in3("3").`,
			facts: map[string][]*fact{
				"out1": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
				"out2": {{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0}},
				"out3": {{[]string{"2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "auto-persistence",
			source: `
Out(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
in1("1","2",L1,0).
in1("3","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"Out": {
					{[]string{"1", "2", "3"}, "L1", 0}, {[]string{"3", "2", "3"}, "L1", 0},
					{[]string{"1", "2", "3"}, "L1", 1}, {[]string{"3", "2", "3"}, "L1", 1},
				},
			},
		},
		{
			msg: "relation without any actual data",
			source: `
out(b,c,l,t) :- in1(l,t), in2(b,c,l,t)
in1(L1,0).
in1(L1,1).
in2("2","3",L1,0).
in2("1","2",L1,0).
in2("1","5",L2,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"1", "2"}, "L1", 0}, {[]string{"2", "3"}, "L1", 0}},
			},
		},
		{
			msg: "constant terms",
			source: `
out(a,l,t) :- in(a,3,l,t)
in("1","2",L1,0).
in("2","3",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"2"}, "L1", 0}},
			},
		},
		{
			msg: "join with constant terms",
			source: `
out(a,b,l,t) :- in1(a,b,l,t), in2(2,b,l,t)
in1("1","2",L1,0).
in1("3","3",L1,0).
in2("2","3",L1,0).
in2("2","4",L1,0).
in2("1","2",L1,0).`,
			facts: map[string][]*fact{
				"out": {{[]string{"3", "3"}, "L1", 0}},
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
				if _, ok := r.relations[rel]; !ok {
					t.Errorf("the relation %q was not found in the engine state", rel)
					continue
				}
				got := r.relations[rel].allAcrossSpaceTime()

				if diff := cmp.Diff(got, want, cmp.AllowUnexported(fact{}), cmpopts.SortSlices(lessFacts)); diff != "" {
					t.Errorf("fact diff for relation %q (-got, +want):\n%s", rel, diff)
				}
			}
		})
	}
}
