package engine

import (
	"strconv"

	"github.com/rithvikp/dedalus/ast"
)

type fact struct {
	data []string
}

type relation struct {
	id    string
	rules []*rule

	indexes []map[string][]*fact
}

type attribute struct {
	relation *relation
	index    int
}

type variable struct {
	id    string
	attrs []*attribute
}

type rule struct {
	id   string
	head *relation
	body []*relation

	// The index in the head relation mapped to the corresponding variable in the body.
	headVarMapping []*variable
	joinVars       []*variable
}

type component struct {
	rules []*rule
}

type Runner struct {
	relations map[string]*relation

	components []*component
}

func NewRunner(p *ast.Program) *Runner {
	runner := Runner{
		relations:  map[string]*relation{},
		components: []*component{{}}, // There is always only a single component for now
	}

	addRel := func(atom *ast.Atom, rl *rule) *relation {
		id := atom.Name
		var ok bool
		var r *relation
		if r, ok = runner.relations[id]; !ok {
			r = &relation{
				id:      id,
				rules:   []*rule{rl},
				indexes: make([]map[string][]*fact, len(atom.Variables)),
			}
			for i := range r.indexes {
				r.indexes[i] = map[string][]*fact{}
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
		runner.components[0].rules = append(runner.components[0].rules, r)

		headVars := map[string]int{}
		for j, v := range astRule.Head.Variables {
			headVars[v.Name] = j
		}

		vars := map[string]*variable{}

		for _, astAtom := range astRule.Body {
			rel := addRel(&astAtom, r)
			r.body = append(r.body, rel)

			for j, astVar := range astAtom.Variables {
				a := &attribute{
					index:    j,
					relation: rel,
				}

				if v, ok := vars[astVar.Name]; ok {
					v.attrs = append(v.attrs, a)
				} else {
					v = &variable{
						id:    astVar.Name,
						attrs: []*attribute{a},
					}
					vars[v.id] = v
					if index, ok := headVars[v.id]; ok {
						r.headVarMapping[index] = v
					}
				}
			}
		}

		for _, v := range vars {
			if len(v.attrs) > 1 {
				r.joinVars = append(r.joinVars, v)
			}
		}
	}

	runner.relations["in1"].push([]string{"a", "b"})
	runner.relations["in1"].push([]string{"f", "b"})
	runner.relations["in2"].push([]string{"b", "c"})
	runner.relations["in2"].push([]string{"b", "e"})

	return &runner
}

func (r *Runner) Step() {
	var queue []*rule
	for _, c := range r.components {
		queue = append(queue, c.rules...)
	}

	for len(queue) != 0 {
		r := queue[0]
		queue = queue[1:]

		modified := join(r.head, r.joinVars, r.headVarMapping)
		if modified {
			queue = append(queue, r.head.rules...)
		}
	}
}

func (r *relation) push(d []string) {
	if r.contains(d) {
		return
	}

	f := &fact{
		data: d,
	}

	for i := range d {
		r.indexes[i][d[i]] = append(r.indexes[i][d[i]], f)
	}
}

func (r *relation) contains(d []string) bool {
	if len(r.indexes) != len(d) {
		return false
	}

	var factSet []*fact
	var ok bool
	if factSet, ok = r.indexes[0][d[0]]; !ok {
		return false
	}

	for _, f := range factSet {
		found := true
		for i := range d {
			if f.data[i] != d[i] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}

	return false
}
