package deps

import (
	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/slices"
)

type CDMap struct {
	Dom   *engine.Relation
	Codom *engine.Relation
}

func CDs(s *engine.State) map[CDMap]*SetFunc[FD] {
	cds := map[CDMap]*SetFunc[FD]{}
	fds := FDs(s)

	for _, rl := range s.Rules() {
		// TODO: Clean up iteration over head + body (instead of this inline append)
		for _, domRel := range append([]*engine.Relation{rl.Head()}, rl.Body()...) {
			relInRuleCDs := cdsForRelInRule(domRel, rl, fds)
			for _, codomRel := range append([]*engine.Relation{rl.Head()}, rl.Body()...) {
				cdMap := CDMap{Dom: domRel, Codom: codomRel}
				newlyAdded := false
				if _, ok := cds[cdMap]; !ok {
					cds[cdMap] = &SetFunc[FD]{equal: fdEqual}
					newlyAdded = true
				}

				if newCDs, ok := relInRuleCDs[codomRel]; ok {
					if newlyAdded {
						cds[cdMap].Union(newCDs)
					} else {
						cds[cdMap].Intersect(newCDs)
					}
				}
			}
		}
	}

	return cds
}

func cdsForRelInRule(domRel *engine.Relation, rl *engine.Rule, fds map[*engine.Relation]*SetFunc[FD]) map[*engine.Relation]*SetFunc[FD] {
	newRDeps := map[*engine.Relation]*SetFunc[FD]{}
	for _, fd := range DepClosure(rl, fds, true).Elems() {
		if !sliceSubset(domRel.Attrs(), fd.Dom) {
			continue
		}
		for _, codomRel := range append([]*engine.Relation{rl.Head()}, rl.Body()...) {
			if codomRel == domRel {
				continue
			}

			if slices.Contains(codomRel.Attrs(), fd.Codom) {
				// TODO: Redesign set func so that the default value is useful
				if _, ok := newRDeps[codomRel]; !ok {
					newRDeps[codomRel] = &SetFunc[FD]{equal: fdEqual}
				}
				newRDeps[codomRel].Add(fd)
			}
		}
	}
	return newRDeps
}

func sliceSubset[T comparable](base []T, subset []T) bool {
	for _, t := range subset {
		if !slices.Contains(base, t) {
			return false
		}
	}
	return true
}
