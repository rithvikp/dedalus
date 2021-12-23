package engine

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/rithvikp/dedalus/ast"
)

type SemanticError struct {
	Position lexer.Position
	Message  string
}

func (e *SemanticError) Error() string {
	return fmt.Sprintf("semantic error at %s: %s", e.Position.String(), e.Message)
}

func newSemanticError(msg string, pos lexer.Position) *SemanticError {
	return &SemanticError{Position: pos, Message: msg}
}

type timeModel int

const (
	timeModelSame = iota
	timeModelSuccessor
	timeModelAsync
)

const (
	successorRelationName = "successor"
	chooseRelationName    = "choose"
)

var lateHandleAtoms = map[string]struct{}{
	successorRelationName: {},
	chooseRelationName:    {},
}

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
	id        string
	bodyRules []*rule

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

	timeModel timeModel
	bodyLoc   string
	headLoc   string

	bodyTimeVar string
	headTimeVar string
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

func NewRunner(p *ast.Program) (*Runner, error) {
	runner := Runner{
		relations: map[string]*relation{},
	}

	addRel := func(atom *ast.Atom, head bool, rl *rule) (*relation, error) {
		id := atom.Name
		var ok bool
		var r *relation
		if r, ok = runner.relations[id]; !ok {
			r = &relation{
				id:      id,
				indexes: make([]map[string]map[locTime][]*fact, len(atom.Variables)-2),
			}
			for i := range r.indexes {
				r.indexes[i] = map[string]map[locTime][]*fact{}
			}
			runner.relations[id] = r
			if !head {
				r.bodyRules = append(r.bodyRules, rl)
			}
		} else {
			if len(r.indexes) != len(atom.Variables)-2 {
				return nil, newSemanticError("the number of attributes must be constant for any given relation", atom.Pos)
			}
			if !head {
				r.bodyRules = append(r.bodyRules, rl)
			}
		}

		return r, nil
	}

	for i, astRule := range p.Rules {
		r := &rule{
			id:             strconv.Itoa(i),
			headVarMapping: make([]*variable, len(astRule.Head.Variables)-2),
		}

		astHeadVars := astRule.Head.Variables
		if len(astHeadVars) < 2 {
			return nil, newSemanticError("all non-replicated read-only relations must have time and location attributes", astRule.Head.Pos)
		}

		var err error
		r.head, err = addRel(&astRule.Head, true, r)
		if err != nil {
			return nil, err
		}
		r.headLoc = astHeadVars[len(astHeadVars)-2].Name
		r.headTimeVar = astHeadVars[len(astHeadVars)-1].Name

		runner.rules = append(runner.rules, r)

		headVars := map[string][]int{}
		for j, v := range astRule.Head.Variables {
			headVars[v.Name] = append(headVars[v.Name], j)
		}

		vars := map[string]*variable{}
		r.vars = map[string][]*variable{}

		r.bodyLoc = ""
		var lateAtoms []*ast.Atom
		for _, astAtom := range astRule.Body {
			if _, ok := lateHandleAtoms[astAtom.Name]; ok {
				lateAtoms = append(lateAtoms, &astAtom)
				continue
			}

			// TODO: Cleanup time/loc parsing (there are many -2's when looking at # of variables
			// due to this issue).
			if len(astAtom.Variables) < 2 {
				return nil, newSemanticError("all non-replicated read-only relations must have time and location attributes", astRule.Head.Pos)
			}

			rel, err := addRel(&astAtom, false, r)
			if err != nil {
				return nil, err
			}

			r.body = append(r.body, rel)
			r.vars[astAtom.Name] = make([]*variable, len(astAtom.Variables)-2)

			atomLoc := astAtom.Variables[len(astAtom.Variables)-2].Name
			if r.bodyLoc == "" {
				r.bodyLoc = atomLoc
			} else if atomLoc != r.bodyLoc {
				return nil, newSemanticError("the location in all body atoms (where applicable) must be the same", astRule.Pos)
			}

			atomTime := astAtom.Variables[len(astAtom.Variables)-1].Name
			if r.bodyTimeVar == "" {
				r.bodyTimeVar = atomTime
			} else if atomTime != r.bodyTimeVar {
				return nil, newSemanticError("the time in all body atoms (where applicable) must be the same", astRule.Pos)
			}

			for j, astVar := range astAtom.Variables {
				if j >= len(astAtom.Variables)-2 {
					break
				}

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

		for _, astAtom := range lateAtoms {
			switch astAtom.Name {
			case successorRelationName:
				if len(astAtom.Variables) != 2 || astAtom.Variables[0].Name != r.bodyTimeVar || astAtom.Variables[1].Name != r.headTimeVar {
					return nil, newSemanticError("incorrectly formatted successor relation", astAtom.Pos)
				}
				r.timeModel = timeModelSuccessor

			case chooseRelationName:
				// TODO: Validate + switch to the correct syntax
				if len(astAtom.Variables) != 2 {
					return nil, newSemanticError("choose relations must have exactly two attributes", astAtom.Pos)
				}

				t := astAtom.Variables[0].NameTuple
				if len(t) != len(r.headVarMapping) {
					return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
				}

				for i, v := range r.headVarMapping {
					if v.id != t[i].Name {
						return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", t[i].Pos)
					}
				}
				r.timeModel = timeModelAsync
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

	return &runner, nil
}

func (r *Runner) Step() {
	var queue []*rule
	inQueue := map[string]struct{}{}
	queue = append(queue, r.rules...)

	for _, rl := range queue {
		inQueue[rl.id] = struct{}{}
	}

	for len(queue) != 0 {
		rl := queue[0]
		delete(inQueue, rl.id)
		queue = queue[1:]

		time := r.currentTimestamp
		loc := rl.bodyLoc
		nextLoc := rl.headLoc

		var nextTime int
		switch rl.timeModel {
		case timeModelSame:
			nextTime = time
		case timeModelSuccessor:
			nextTime = time + 1
		case timeModelAsync:
			nextTime = time + rand.Intn(5) // TODO
		}

		modified := join(rl, loc, time, nextLoc, nextTime)
		if modified {
			for _, bodyRule := range rl.head.bodyRules {
				if _, ok := inQueue[bodyRule.id]; !ok {
					queue = append(queue)
				}
			}
		}
	}

	r.currentTimestamp++
}
