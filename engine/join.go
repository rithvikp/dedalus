package engine

import "fmt"

type factNode struct {
	lockedVars map[*variable]string
}

// This is EXTREMELY hacky/non-clean code, but it works as a proof of concept.
func join(rl *rule, loc string, time int, nextLoc string, nextTime int) bool {
	lt := locTime{loc, time}

	var fringe []*factNode
	rel := rl.body[0]
	for _, f := range rel.all(loc, time) {
		fn := &factNode{lockedVars: map[*variable]string{}}

		consistent := true
		for _, v := range rl.vars[rel.id] {
			attrs := v.attrs[rel.id]
			val := f.data[attrs[0].index]
			for _, a := range attrs {
				if f.data[a.index] != val {
					consistent = false
				}
			}
			fn.lockedVars[v] = val
			if !consistent {
				break
			}
		}

		if consistent {
			fringe = append(fringe, fn)
		}
	}

	addChildren := func(node *factNode, rel *relation) []*factNode {
		first := true
		var workingSet []*fact
		for _, v := range rl.vars[rel.id] {
			if val, ok := node.lockedVars[v]; ok {
				for _, a := range v.attrs[rel.id] {
					matched, ok := rel.indexes[a.index][val][lt]
					if !ok {
						return nil
					}
					var newWorkingSet []*fact
					if first {
						newWorkingSet = matched
					} else {
						// TODO: Switched to a hashed setup
						for _, f1 := range workingSet {
							for _, f2 := range matched {
								if f1.equals(f2) {
									newWorkingSet = append(newWorkingSet, f1)
								}
							}
						}
					}
					workingSet = newWorkingSet
				}
			}
			first = false
		}

		var children []*factNode
		for _, f := range workingSet {
			fn := &factNode{lockedVars: map[*variable]string{}}
			for k, v := range node.lockedVars {
				fn.lockedVars[k] = v
			}

			consistent := true
			for _, v := range rl.vars[rel.id] {
				attrs := v.attrs[rel.id]
				val := f.data[attrs[0].index]
				for _, a := range attrs {
					if f.data[a.index] != val {
						consistent = false
					}
				}
				fn.lockedVars[v] = val
				if !consistent {
					break
				}
			}
			if consistent {
				children = append(children, fn)
			}
		}
		return children
	}

	for i := 1; i < len(rl.body); i++ {
		rel := rl.body[i]
		var nextFringe []*factNode
		for _, parent := range fringe {
			nextFringe = append(nextFringe, addChildren(parent, rel)...)
		}

		fringe = nextFringe
	}

	modified := false
	for _, fn := range fringe {
		d := make([]string, len(rl.head.indexes))
		for i, v := range rl.headVarMapping {
			d[i] = fn.lockedVars[v]
		}
		fmt.Println(rl.head.id+":", d, nextLoc, nextTime)
		if rl.head.push(d, nextLoc, nextTime) {
			modified = true
		}
	}

	return modified
}
