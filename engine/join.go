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

	for _, fn := range fringe {
		d := make([]string, len(rl.head.indexes))
		for i, v := range rl.headVarMapping {
			d[i] = fn.lockedVars[v]
		}
		fmt.Println(d)
		rl.head.push(d, nextLoc, nextTime)
	}

	//do := func(n *factNode, relIndex int, v *variable) ([]*factNode, bool) {
	//a := v.attrs[relIndex]

	//factSet, ok := v.attrs[relIndex].relation.indexes[a.index][n.f.data[n.a.index]][lt]
	//if !ok {
	//return nil, false
	//}

	//nodes := make([]*factNode, len(factSet))
	//for i, f := range factSet {
	//fn := &factNode{
	//f: f,
	//a: a,
	//}

	//n.children = append(n.children, fn)
	//nodes[i] = fn
	//}

	//return nodes, true
	//}

	//var roots []*factNode
	//for i := 0; i < len(joinVars); i++ {
	//v := joinVars[i]
	//a := v.attrs[0]
	//for _, f := range a.relation.all(loc, time) {
	//node := &factNode{
	//f: f,
	//a: a,
	//}
	//roots = append(roots, node)
	//}

	//fringe := roots
	//for j := 1; j < len(v.attrs); j++ {
	//a = v.attrs[j]
	//var nextFringe []*factNode
	//for k := 0; k < len(fringe); k++ {
	//node := fringe[k]
	//children, ok := do(node, j, v)
	//if !ok {
	//fringe = append(fringe[:k], fringe[k+1:]...)
	//k--
	//continue
	//}
	//nextFringe = append(nextFringe, children...)
	//}
	//fringe = nextFringe
	//}
	//}

	//for _, node := range roots {
	//traverseFactTree(node, nil, head, headVarMapping, nextLoc, nextTime)
	//}

	return false
}

//func traverseFactTree(n *factNode, head *relation, headVarMapping []*variable, nextLoc string, nextTime int) {
//if len(n.children) == 0 {
//d := make([]string, len(head.indexes))
//for i, v := range headVarMapping {
//d[i] = n.lockedVars[v]
//}
//fmt.Println(d)
//head.push(d, nextLoc, nextTime)
//return
//}

//for _, c := range n.children {
//traverseFactTree(c, head, headVarMapping, nextLoc, nextTime)
//}
//}
