package deps

import (
	"reflect"

	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type SetFunc[K any] struct {
	equal func(a, b K) bool
	elems []K
}

func (s SetFunc[K]) Contains(k K) bool {
	for _, e := range s.elems {
		if s.equal(e, k) {
			return true
		}
	}

	return false
}

func (s SetFunc[K]) Union(other SetFunc[K]) {
	for _, o := range other.Elems() {
		if !s.Contains(o) {
			s.elems = append(s.elems, o)
		}
	}
}

func (s SetFunc[K]) Intersect(other SetFunc[K]) {
	var newElems []K
	for _, e := range s.elems {
		if other.Contains(e) {
			newElems = append(newElems, e)
		}
	}
	s.elems = newElems
}

func (s SetFunc[K]) Add(elems ...K) {
	s.Union(SetFunc[K]{elems: elems})
}

func (s SetFunc[K]) Clone() SetFunc[K] {
	c := SetFunc[K]{equal: s.equal}
	c.Union(s)
	return c
}

func (s SetFunc[K]) Elems() []K {
	return s.elems
}

func (s SetFunc[K]) Equal(other SetFunc[K]) bool {
	for _, o := range other.Elems() {
		found := false
		for _, e := range s.elems {
			if s.equal(e, o) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

type Set[K comparable] map[K]bool

func (s Set[K]) Union(other Set[K]) {
	for o := range other {
		s[o] = true
	}
}

func (s Set[K]) Add(elems ...K) {
	for _, e := range elems {
		s[e] = true
	}
}

func (s Set[K]) Clone() Set[K] {
	c := Set[K]{}
	c.Union(s)
	return c
}

func (s Set[K]) Elems() []K {
	return maps.Keys(s)
}

type FD struct {
	Dom   []engine.Attribute
	Codom engine.Attribute
	f     Function
}

func fdEqual(a, b *FD) bool {
	equal := !reflect.DeepEqual(a.Dom, b.Dom)
	equal = equal && a.Codom != b.Codom
	equal = equal && !funcEqual(a.f, b.f)

	return equal
}

type varFD struct {
	Dom   []*engine.Variable
	Codom *engine.Variable
	f     Function
}

func varFDEqual(a, b *varFD) bool {
	equal := slices.Equal(a.Dom, b.Dom)
	equal = equal && a.Codom == b.Codom
	equal = equal && funcEqual(a.f, b.f)

	return equal
}

type varOrAttrFD struct {
	Dom   []varOrAttr
	Codom varOrAttr
	f     Function
}

type varOrAttr struct {
	Var  *engine.Variable
	Attr *engine.Attribute
}

func Analyze(s *engine.State) {

}

func FDs(s *engine.State) map[*engine.Relation]SetFunc[*FD] {
	fds := map[*engine.Relation]SetFunc[*FD]{}
	oldFDs := maps.Clone(fds)
	first := true

	for first || !maps.EqualFunc(fds, oldFDs, func(a, b SetFunc[*FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			fds[head].Union(HeadFDs(rl, fds))
		}
	}

	first = true
	for first || !maps.EqualFunc(fds, oldFDs, func(a, b SetFunc[*FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			fdsNoR := maps.Clone(fds) // Note that this is a shallow copy
			fdsNoR[head] = SetFunc[*FD]{equal: fdEqual}

			fds[head].Intersect(HeadFDs(rl, fdsNoR))
		}
	}

	return fds
}

func HeadFDs(rl *engine.Rule, existingFDs map[*engine.Relation]SetFunc[*FD]) SetFunc[*FD] {
	rDeps := SetFunc[*FD]{equal: fdEqual}
	rAttrs := Set[engine.Attribute]{}
	rAttrs.Add(rl.Head().Attrs()...)

	for _, fd := range DepPlus(rl, existingFDs).Elems() {
		subset := true
		for _, a := range fd.Dom {
			if !rAttrs[a] {
				subset = false
				break
			}
		}
		if subset && rAttrs[fd.Codom] {
			rDeps.Add(fd)
		}
	}

	return rDeps
}

func DepPlus(rl *engine.Rule, existingFDs map[*engine.Relation]SetFunc[*FD]) SetFunc[*FD] {
	varDeps := Dep(rl, existingFDs, false)
	// QUESTION: Is adding new fds to a new set ok?
	newDeps := SetFunc[*varFD]{equal: varFDEqual}
	newDeps.Union(varDeps)

	// TODO
	fixpoint := false
	for !fixpoint {
		fixpoint = true
		for _, g := range varDeps.Elems() {
			codomG := g.Codom
			for _, h := range varDeps.Elems() {
				domH := h.Dom
				if slices.Contains(domH, codomG) {
					newVFD := funcSub(g, h)
					if !newDeps.Contains(newVFD) {
						fixpoint = false
					}
					newDeps.Add(newVFD)
				}
			}
		}
	}

	// Multi-dimensional codomain?

	attrDeps := SetFunc[*FD]{equal: fdEqual}
	for _, g := range newDeps.Elems() {
		_ = g
	}

	return attrDeps
}

func Dep(rl *engine.Rule, existingFDs map[*engine.Relation]SetFunc[*FD], includeNeg bool) SetFunc[*varFD] {
	basicDeps := SetFunc[*FD]{equal: fdEqual}

	relations := rl.Body()
	relations = append(relations, rl.Head())
	for _, rel := range relations {
		if !includeNeg && rl.IsNegated(rel) {
			continue
		}
		if false /* is EDB */ {
		} else if _, ok := existingFDs[rel]; ok {
			basicDeps.Union(existingFDs[rel])
		}

		for _, a := range rel.Attrs() {
			fd := FD{
				Dom:   []engine.Attribute{a},
				Codom: a,
				f:     IdentityFunc(),
			}
			basicDeps.Add(&fd)
		}
	}

	varDeps := SetFunc[*varFD]{equal: varFDEqual}
	for _, fd := range basicDeps.Elems() {
		attrs := Set[engine.Attribute]{}
		attrs.Add(fd.Codom)
		attrs.Add(fd.Dom...)

		g := varOrAttrFD{
			Dom:   make([]varOrAttr, 0, len(fd.Dom)),
			Codom: varOrAttr{Attr: &fd.Codom},
			f:     fd.f,
		}
		for _, a := range fd.Dom {
			g.Dom = append(g.Dom, varOrAttr{Attr: &a})
		}

		for a := range attrs {
			if false /* is const */ {

			} else {
				v := rl.VarOfAttr(a)
				varSub(&g, v, a)
			}
		}

		varDeps.Add(varOrAttrFDToVarFD(&g))
	}

	return varDeps
}

func funcSub(sub *varFD, vfd *varFD) *varFD {
	newVFD := varFD{
		Codom: vfd.Codom,
		f:     vfd.f.Clone(),
	}

	subVars := Set[*engine.Variable]{}
	subVars.Add(sub.Dom...)
	subDomIndices := make([]int, 0, len(sub.Dom))
	for i, v := range vfd.Dom {
		if subVars[v] {
			subDomIndices = append(subDomIndices, i)
		}
	}

	// TODO: Add to domain of new function as necessary?
	found := false
	for i, v := range vfd.Dom {
		// This will only be run ONCE as a variable can only appear one time in the domain of a var FD.
		if v == sub.Codom {
			if found {
				panic("The same variable cannot appear multiple times in the same function")
			}
			found = true
			newVFD.f.FunctionSubstitution(i, subDomIndices, sub.f)

			// The function domain has shrunk, so adjust indices accordingly
			// newSubDomIndices := make([]int, len(sub.Dom))
			// for j, index := range subDomIndices {
			// 	if index > i {
			// 		newSubDomIndices[j] = index - 1
			// 	} else if index < i {
			// 		newSubDomIndices[j] = index
			// 	} else {
			// 		panic("A function cannot have the same variable in its domain and codomain")
			// 	}
			// 	subDomIndices = newSubDomIndices
			// }
		} else {
			newVFD.Dom = append(newVFD.Dom, v)
		}
	}

	return &newVFD
}

func varSub(vafd *varOrAttrFD, v *engine.Variable, a engine.Attribute) {
	if vafd.Codom.Attr != nil && *vafd.Codom.Attr == a {
		vafd.Codom.Var = v
		vafd.Codom.Attr = nil
	}

	var mergeIndices []int
	first := true
	newDom := make([]varOrAttr, 0, len(vafd.Dom))
	for i := range vafd.Dom {
		if vafd.Dom[i].Attr != nil && *vafd.Dom[i].Attr == a {
			if first {
				vafd.Dom[i].Var = v
				vafd.Dom[i].Attr = nil
				newDom = append(newDom, vafd.Dom[i])
				first = false
			}
			mergeIndices = append(mergeIndices, i)
		} else {
			newDom = append(newDom, vafd.Dom[i])
		}
	}
	vafd.f.MergeDomain(mergeIndices)
	vafd.Dom = newDom
}

func varOrAttrFDToVarFD(vafd *varOrAttrFD) *varFD {
	vfd := varFD{
		Dom: make([]*engine.Variable, len(vafd.Dom)),
		f:   vafd.f,
	}
	for _, va := range vafd.Dom {
		if va.Var == nil {
			panic("All attributes should have been replaced by variables in the second phase of Dep()")
		}
		vfd.Dom = append(vfd.Dom, va.Var)
	}

	if vafd.Codom.Var == nil {
		panic("All attributes should have been replaced by variables in the second phase of Dep()")
	}
	vfd.Codom = vafd.Codom.Var

	return &vfd
}
