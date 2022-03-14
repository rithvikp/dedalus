package deps

import (
	"fmt"
	"strings"

	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type SetFunc[K any] struct {
	equal func(a, b K) bool
	elems []K
}

func (s *SetFunc[K]) Contains(k K) bool {
	for _, e := range s.elems {
		if s.equal(e, k) {
			return true
		}
	}

	return false
}

func (s *SetFunc[K]) Union(other *SetFunc[K]) {
	for _, o := range other.Elems() {
		if !s.Contains(o) {
			s.elems = append(s.elems, o)
		}
	}
}

func (s *SetFunc[K]) Intersect(other *SetFunc[K]) {
	var newElems []K
	for _, e := range s.elems {
		if other.Contains(e) {
			newElems = append(newElems, e)
		}
	}
	s.elems = newElems
}

func (s *SetFunc[K]) Add(elems ...K) {
	s.Union(&SetFunc[K]{elems: elems})
}

func (s *SetFunc[K]) Clone() *SetFunc[K] {
	c := &SetFunc[K]{equal: s.equal}
	c.Union(s)
	return c
}

func (s *SetFunc[K]) Elems() []K {
	return s.elems
}

func (s *SetFunc[K]) Equal(other *SetFunc[K]) bool {
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

func (s Set[K]) Delete(elem K) {
	delete(s, elem)
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

func (fd FD) String() string {
	b := strings.Builder{}
	b.WriteString("{ Dom: [")
	for i, a := range fd.Dom {
		b.WriteString(a.String())
		if i < len(fd.Dom)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("], Codom: ")
	b.WriteString(fd.Codom.String())
	b.WriteString(", Func: ")
	b.WriteString(fd.f.String())
	b.WriteString(" }")

	return b.String()
}

func fdEqual(a, b *FD) bool {
	equal := slices.Equal(a.Dom, b.Dom)
	equal = equal && a.Codom == b.Codom
	equal = equal && funcEqual(a.f, b.f)

	return equal
}

func (fd *FD) Reflexive() bool {
	return len(fd.Dom) == 1 && fd.Dom[0] == fd.Codom
}

type varFD struct {
	Dom   []*engine.Variable
	Codom *engine.Variable
	f     Function
}

func (fd varFD) String() string {
	b := strings.Builder{}
	b.WriteString("{ Dom: [")
	for i, v := range fd.Dom {
		b.WriteString(v.String())
		if i < len(fd.Dom)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("], Codom: ")
	b.WriteString(fd.Codom.String())
	b.WriteString(", Func: ")
	b.WriteString(fd.f.String())
	b.WriteString(" }")

	return b.String()
}

func (v *varFD) Reflexive() bool {
	return len(v.Dom) == 1 && v.Dom[0] == v.Codom
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

func varOrAttrFDEqual(a, b *varOrAttrFD) bool {
	equal := slices.Equal(a.Dom, b.Dom)
	equal = equal && a.Codom == b.Codom
	equal = equal && funcEqual(a.f, b.f)

	return equal
}

func (v *varOrAttrFD) Clone() *varOrAttrFD {
	n := &varOrAttrFD{
		Dom:   slices.Clone(v.Dom),
		Codom: v.Codom,
		f:     v.f.Clone(),
	}
	return n
}

type varOrAttr struct {
	Var  *engine.Variable
	Attr *engine.Attribute
}

func Analyze(s *engine.State) {

}

func FDs(s *engine.State) map[*engine.Relation]*SetFunc[*FD] {
	fds := map[*engine.Relation]*SetFunc[*FD]{}
	oldFDs := maps.Clone(fds)
	first := true

	for first || !maps.EqualFunc(fds, oldFDs, func(a, b *SetFunc[*FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			if _, ok := fds[head]; !ok {
				fds[head] = &SetFunc[*FD]{equal: fdEqual}
			}
			fds[head].Union(HeadFDs(rl, fds))
		}
	}

	first = true
	for first || !maps.EqualFunc(fds, oldFDs, func(a, b *SetFunc[*FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			fdsNoR := maps.Clone(fds) // Note that this is a shallow copy
			fdsNoR[head] = &SetFunc[*FD]{equal: fdEqual}

			fds[head].Intersect(HeadFDs(rl, fdsNoR))
		}
	}

	finalFDs := map[*engine.Relation]*SetFunc[*FD]{}
	for rel, deps := range fds {
		finalFDs[rel] = &SetFunc[*FD]{equal: fdEqual}
		for _, fd := range deps.Elems() {
			if !fd.Reflexive() {
				finalFDs[rel].Add(fd)
			}
		}
	}

	return finalFDs
}

func HeadFDs(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[*FD]) *SetFunc[*FD] {
	rDeps := &SetFunc[*FD]{equal: fdEqual}
	rAttrs := Set[engine.Attribute]{}
	rAttrs.Add(rl.Head().Attrs()...)

	for _, fd := range DepClosure(rl, existingFDs).Elems() {
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

func DepClosure(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[*FD]) *SetFunc[*FD] {
	varDeps := Dep(rl, existingFDs, false)
	newDeps := &SetFunc[*varFD]{equal: varFDEqual}
	newDeps.Union(varDeps)

	fixpoint := false
	for !fixpoint {
		fixpoint = true
		for _, g := range varDeps.Elems() {
			if g.Reflexive() {
				continue
			}
			codomG := g.Codom
			for _, h := range varDeps.Elems() {
				if varFDEqual(g, h) || h.Reflexive() {
					continue
				}
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

	attrDeps := &SetFunc[*FD]{equal: fdEqual}
	for _, vfd := range newDeps.Elems() {
		vafds := &SetFunc[*varOrAttrFD]{equal: varOrAttrFDEqual}
		f := &varOrAttrFD{
			Dom:   make([]varOrAttr, len(vfd.Dom)),
			Codom: varOrAttr{Var: vfd.Codom},
			f:     vfd.f,
		}
		for i, v := range vfd.Dom {
			f.Dom[i] = varOrAttr{Var: v}
		}
		vafds.Add(f)

		for _, v := range vfd.Dom {
			newVafds := &SetFunc[*varOrAttrFD]{equal: varOrAttrFDEqual}
			for _, vafd := range vafds.Elems() {
				for _, a := range v.Attrs() {
					g := vafd.Clone()
					attrDomSub(g, a, v)
					newVafds.Add(g)
				}
			}
			vafds = newVafds
		}

		v := vfd.Codom
		newVafds := &SetFunc[*varOrAttrFD]{equal: varOrAttrFDEqual}
		for _, vafd := range vafds.Elems() {
			for _, a := range v.Attrs() {
				g := vafd.Clone()
				attrCodomSub(g, a, v)
				newVafds.Add(g)
			}
		}
		vafds = newVafds

		for _, vafd := range vafds.Elems() {
			attrDeps.Add(varOrAttrFDToFD(vafd))
		}
	}

	return attrDeps
}

func Dep(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[*FD], includeNeg bool) *SetFunc[*varFD] {
	basicDeps := &SetFunc[*FD]{equal: fdEqual}

	relations := rl.Body()
	relations = append(relations, rl.Head())
	for _, rel := range relations {
		if !includeNeg && rl.IsNegated(rel) {
			continue
		}
		if rel.IsEDB() {
			// TODO: Rewrite and clean EDB handling
			switch rel.ID() {
			case "add":
				fd := &FD{
					Dom:   []engine.Attribute{rel.Attrs()[0], rel.Attrs()[1]},
					Codom: rel.Attrs()[2],
					f:     ExprFunc(AddExp(IdentityExp(0), IdentityExp(1)), 2),
				}
				basicDeps.Add(fd)
			case "sub":
				fd := &FD{
					Dom:   []engine.Attribute{rel.Attrs()[0], rel.Attrs()[1]},
					Codom: rel.Attrs()[2],
					f:     ExprFunc(SubExp(IdentityExp(0), IdentityExp(1)), 2),
				}
				basicDeps.Add(fd)
			}
		} else if _, ok := existingFDs[rel]; ok && rel != rl.Head() {
			basicDeps.Union(existingFDs[rel])
			// QUESTION: Filter out existing fd's for the head?
		}

		// Add reflexive fd's
		for _, a := range rel.Attrs() {
			// Skip constant attributes
			if _, ok := rl.ConstOfAttr(a); ok {
				continue
			}

			fd := &FD{
				Dom:   []engine.Attribute{a},
				Codom: a,
				f:     IdentityFunc(),
			}
			basicDeps.Add(fd)
		}
	}

	varDeps := &SetFunc[*varFD]{equal: varFDEqual}
	for _, fd := range basicDeps.Elems() {
		attrs := Set[engine.Attribute]{}
		attrs.Add(fd.Codom)
		attrs.Add(fd.Dom...)

		g := varOrAttrFD{
			Dom:   make([]varOrAttr, len(fd.Dom)),
			Codom: varOrAttr{Attr: &fd.Codom},
			f:     fd.f,
		}
		for i, a := range fd.Dom {
			a := a
			g.Dom[i] = varOrAttr{Attr: &a}
		}

		for a := range attrs {
			v, notConst := rl.VarOfAttr(a)
			if notConst {
				varSub(&g, v, a)
			} else {
				c, ok := rl.ConstOfAttr(a)
				if !ok {
					panic(fmt.Sprintf("Found an attribute %q which was not a variable or a constant", a))
				}
				constSub(&g, c, a)
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
			// A variable will only appear once in the domain
			subVars.Delete(v)
		}
	}

	newVars := make([]*engine.Variable, 0, len(subVars.Elems()))
	for i, v := range sub.Dom {
		if subVars[v] {
			subDomIndices = append(subDomIndices, len(vfd.Dom)+i)
			newVars = append(newVars, v)
		}
	}

	found := false
	for i, v := range vfd.Dom {
		// This will only be run ONCE as a variable can only appear one time in the domain of a var FD.
		if v == sub.Codom {
			if found {
				panic("The same variable cannot appear multiple times in the same functional dependency")
			}
			found = true

			newVFD.f.AddToDomain(len(newVars))
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

	// Only modify the function if a substitution match was found
	if found {
		newVFD.Dom = append(newVFD.Dom, newVars...)
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

func constSub(vafd *varOrAttrFD, val int, a engine.Attribute) {
	for i := 0; i < len(vafd.Dom); i++ {
		if vafd.Dom[i].Attr != nil && *vafd.Dom[i].Attr == a {
			vafd.f.FunctionSubstitution(i, []int{}, ConstFunc(val))
			vafd.Dom = slices.Delete(vafd.Dom, i, i+1)
			i--
		}
	}
}

func attrDomSub(vafd *varOrAttrFD, a engine.Attribute, v *engine.Variable) {
	for i := range vafd.Dom {
		if vafd.Dom[i].Var != nil && vafd.Dom[i].Var == v {
			vafd.Dom[i].Attr = &a
			vafd.Dom[i].Var = nil
		}
	}
}

func attrCodomSub(vafd *varOrAttrFD, a engine.Attribute, v *engine.Variable) {
	if vafd.Codom.Var != nil && vafd.Codom.Var == v {
		vafd.Codom.Attr = &a
		vafd.Codom.Var = nil
	}
}

func varOrAttrFDToVarFD(vafd *varOrAttrFD) *varFD {
	vfd := varFD{
		Dom: make([]*engine.Variable, len(vafd.Dom)),
		f:   vafd.f,
	}
	for i, va := range vafd.Dom {
		if va.Var == nil {
			panic(fmt.Sprintf("All attributes should have been replaced by variables in the second phase of Dep(); %q was not", va.Attr.String()))
		}
		vfd.Dom[i] = va.Var
	}

	if vafd.Codom.Var == nil {
		panic(fmt.Sprintf("All attributes should have been replaced by variables in the second phase of Dep(); %q was not", vafd.Codom.Attr.String()))
	}
	vfd.Codom = vafd.Codom.Var

	return &vfd
}

func varOrAttrFDToFD(vafd *varOrAttrFD) *FD {
	fd := FD{
		Dom: make([]engine.Attribute, len(vafd.Dom)),
		f:   vafd.f,
	}
	for i, va := range vafd.Dom {
		if va.Attr == nil {
			panic("All variables should have been replaced by attributes in the second phase of DepClosure()")
		}
		fd.Dom[i] = *va.Attr
	}

	if vafd.Codom.Attr == nil {
		panic("All variables should have been replaced by attributes in the second phase of DepClosure()")
	}
	fd.Codom = *vafd.Codom.Attr

	return &fd
}
