package engine

import (
	"strconv"
)

type factNode struct {
	lockedVars map[*variable]string
}

func join(rl *rule, loc string, time int) [][]string {
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
		var workingSet []*fact
		first := true
		for _, v := range rl.vars[rel.id] {
			var newWorkingSet []*fact
			for _, a := range v.attrs[rel.id] {
				var matched []*fact
				if val, ok := node.lockedVars[v]; ok {
					matched, ok = rel.indexes[a.index][val][lt]
					if !ok {
						return nil
					}
				} else {
					matched = rel.all(loc, time)
				}

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
			}
			workingSet = newWorkingSet
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

				fn.lockedVars[v] = val
				// Ensure time and location constraints are maintained
				if v == rl.bodyLocVar && val != loc {
					consistent = false
				} else if v == rl.bodyTimeVar && val != strconv.Itoa(time) {
					consistent = false
				}
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

	data := make([][]string, len(fringe))
	for i, fn := range fringe {
		d := make([]string, len(rl.head.indexes)+1)
		assignVar := func(v *variable) string {
			if v == rl.bodyLocVar {
				return loc
			} else if v == rl.bodyTimeVar {
				return strconv.Itoa(time)
			} else {
				return fn.lockedVars[v]
			}
		}

		for j, ht := range rl.headVarMapping {
			d[j] = assignVar(ht.v)
		}

		d[len(d)-1] = assignVar(rl.headLocVar)
		data[i] = d
	}

	return data
}
