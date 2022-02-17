package deps

import "github.com/rithvikp/dedalus/engine"

type Set[K comparable] map[K]bool

func (s *Set[K]) Union(other *Set[K]) {
	for k := range other {
		if !s[k] {
			s[k] = true
		}
	}
}

type FunctionalDependency struct {
}

func Analyze(s *engine.State) {

}

func Dep(rl *engine.Rule, existingFDs map[*engine.Relation]Set[*FunctionalDependency]) {
	var basicDep Set[*FunctionalDependency]

	relations := rl.Body()
	relations = append(relations, rl.Head())
	for _, rel := range relations {
		// if isEDB

		if existingFDs[rel] {
			basicDep.Union(existingFDs[rel])
		}

		// TODO: Confirm that this is finding any relationships in this specific rule (as looking at
		// body relationships as well, which may not be true for entire relation)
		//for _
	}
	_ = basicDep
}
