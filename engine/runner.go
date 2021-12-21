package engine

import (
	"strconv"

	"github.com/rithvikp/dedalus/ast"
)

type fact struct {
	data      []string
	location  string
	timestamp int
}

type locTime struct {
	location  string
	timestamp int
}

type relation struct {
	id    string
	rules []*rule

	indexes []map[string]map[locTime][]*fact
}

type attribute struct {
	relation *relation
	index    int
}

type variable struct {
	id    string
	attrs map[string][]*attribute
}

type rule struct {
	id   string
	head *relation
	body []*relation

	// The index in the head relation mapped to the corresponding variable in the body.
	headVarMapping []*variable
	vars           map[string][]*variable
}

type Runner struct {
	relations map[string]*relation

	rules []*rule

	currentTimestamp int
}

func (f *fact) equals(other *fact) bool {
	if len(f.data) != len(other.data) {
		return false
	} else if f.location != other.location {
		return false
	} else if f.timestamp != other.timestamp {
		return false
	}

	for i := range f.data {
		if f.data[i] != other.data[i] {
			return false
		}
	}
	return true
}

func NewRunner(p *ast.Program) *Runner {
	runner := Runner{
		relations: map[string]*relation{},
	}

	addRel := func(atom *ast.Atom, rl *rule) *relation {
		id := atom.Name
		var ok bool
		var r *relation
		if r, ok = runner.relations[id]; !ok {
			r = &relation{
				id:      id,
				rules:   []*rule{rl},
				indexes: make([]map[string]map[locTime][]*fact, len(atom.Variables)),
			}
			for i := range r.indexes {
				r.indexes[i] = map[string]map[locTime][]*fact{}
			}
			runner.relations[id] = r
		} else {
			// TODO: confirm the number of attrs is constant in parsing
			r.rules = append(r.rules, rl)
		}

		return r
	}

	for i, astRule := range p.Rules {
		r := &rule{
			id:             strconv.Itoa(i),
			headVarMapping: make([]*variable, len(astRule.Head.Variables)),
		}
		r.head = addRel(&astRule.Head, r)
		runner.rules = append(runner.rules, r)

		headVars := map[string][]int{}
		for j, v := range astRule.Head.Variables {
			headVars[v.Name] = append(headVars[v.Name], j)
		}

		vars := map[string]*variable{}
		r.vars = map[string][]*variable{}

		for _, astAtom := range astRule.Body {
			rel := addRel(&astAtom, r)
			r.body = append(r.body, rel)
			r.vars[astAtom.Name] = make([]*variable, len(astAtom.Variables))

			for j, astVar := range astAtom.Variables {
				a := &attribute{
					index:    j,
					relation: rel,
				}

				var v *variable
				var ok bool
				if v, ok = vars[astVar.Name]; ok {
					v.attrs[rel.id] = append(v.attrs[rel.id], a)
				} else {
					v = &variable{
						id:    astVar.Name,
						attrs: map[string][]*attribute{rel.id: {a}},
					}
					vars[v.id] = v
				}

				if indices, ok := headVars[v.id]; ok {
					for _, k := range indices {
						r.headVarMapping[k] = v
					}
				}
				r.vars[astAtom.Name][j] = v
			}
		}
	}

	runner.relations["in1"].push([]string{"a", "b"}, "L1", 0)
	runner.relations["in1"].push([]string{"f", "b"}, "L1", 0)
	runner.relations["in2"].push([]string{"b", "c"}, "L1", 0)
	runner.relations["in2"].push([]string{"a", "b"}, "L1", 0)

	//runner.relations["in1"].push([]string{"a", "a"}, "L1", 0)
	//runner.relations["in1"].push([]string{"f", "b"}, "L1", 0)
	//runner.relations["in2"].push([]string{"a", "a"}, "L1", 0)
	//runner.relations["in2"].push([]string{"a", "b"}, "L1", 0)

	return &runner
}

func (r *Runner) Step() {
	var queue []*rule
	queue = append(queue, r.rules...)

	for len(queue) != 0 {
		rl := queue[0]
		queue = queue[1:]

		loc := "L1"
		time := r.currentTimestamp
		nextLoc := "L1"
		nextTime := 0

		modified := join(rl, loc, time, nextLoc, nextTime)
		if modified {
			queue = append(queue, rl.head.rules...)
		}
	}

	r.currentTimestamp++
}
