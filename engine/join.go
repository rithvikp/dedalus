package engine

import "fmt"

type factNode struct {
	f        *fact
	a        *attribute
	children []*factNode
}

// This is EXTREMELY hacky/non-clean code, but it works as a proof of concept.
func join(head *relation, joinVars []*variable, headVarMapping []*variable) bool {
	v := joinVars[0]
	do := func(n *factNode, relIndex int) []*factNode {
		a := v.attrs[relIndex]
		factSet, ok := v.attrs[relIndex].relation.indexes[a.index][n.f.data[n.a.index]]
		if !ok {
			return nil
		}

		nodes := make([]*factNode, len(factSet))
		for i, f := range factSet {
			fn := &factNode{
				f: f,
				a: a,
			}
			n.children = append(n.children, fn)
			nodes[i] = fn
		}

		return nodes
	}

	a := v.attrs[0]
	var roots []*factNode
	for _, factSet := range a.relation.indexes[0] {
		for _, f := range factSet {
			node := &factNode{
				f: f,
				a: a,
			}
			roots = append(roots, node)
		}
	}

	fringe := roots
	for i := 1; i < len(v.attrs); i++ {
		a = v.attrs[i]
		var nextFringe []*factNode
		for _, node := range fringe {
			nextFringe = append(nextFringe, do(node, i)...)
		}
		fringe = nextFringe
	}

	for _, node := range roots {
		traverseFactTree(node, nil, head, headVarMapping)
	}

	return false
}

func traverseFactTree(n *factNode, ancestry []*factNode, head *relation, headVarMapping []*variable) {
	ancestry = append(ancestry, n)
	if len(n.children) == 0 {
		m := map[string]*fact{}
		for _, fn := range ancestry {
			m[fn.a.relation.id] = fn.f
		}

		d := make([]string, len(head.indexes))
		for i, v := range headVarMapping {
			a := v.attrs[0]
			d[i] = m[a.relation.id].data[a.index]
		}
		fmt.Println(d)
		head.push(d)
		return
	}

	for _, c := range n.children {
		traverseFactTree(c, ancestry, head, headVarMapping)
	}

	ancestry = ancestry[:len(ancestry)-1]
}
