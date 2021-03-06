package deps

import (
	"fmt"
	"strings"

	"github.com/rithvikp/dedalus/analysis/fn"
	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type DepIO interface {
	engine.Attribute | *engine.Variable | varOrAttr
	String() string
}

type Dep[IO DepIO] struct {
	Dom           []IO
	Codom         IO
	f             fn.Func
	substitutions map[IO]Dep[IO]
}

func (d Dep[IO]) String() string {
	b := strings.Builder{}
	b.WriteString("{ Dom: [")
	for i, v := range d.Dom {
		b.WriteString(v.String())
		if i < len(d.Dom)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("], Codom: ")
	b.WriteString(d.Codom.String())
	b.WriteString(", Func: ")
	b.WriteString(d.f.String())
	b.WriteString(" }")

	return b.String()
}

func (d Dep[IO]) Reflexive() bool {
	return len(d.Dom) == 1 && d.Dom[0] == d.Codom
}

func (d Dep[IO]) Clone() Dep[IO] {
	return Dep[IO]{
		Dom:           slices.Clone(d.Dom),
		Codom:         d.Codom,
		f:             d.f.Clone(),
		substitutions: maps.Clone(d.substitutions),
	}
}

func (d Dep[IO]) Normalize() Dep[IO] {
	// Normalization is currently only defined for attribute-based dependencies
	// Specialization is unfortunately a little hack-y.
	switch d := (interface{})(&d).(type) {
	case *Dep[engine.Attribute]:
		sortedDom := slices.Clone(d.Dom)
		slices.SortFunc(sortedDom, func(a, b engine.Attribute) bool {
			return a.LessThan(b)
		})
		newPositions := map[engine.Attribute]int{}
		for i, a := range sortedDom {
			newPositions[a] = i
		}

		replacements := map[int]fn.Expression{}
		for i := 0; i < len(d.Dom); i++ {
			newIndex := newPositions[d.Dom[i]]
			if newIndex != i {
				replacements[i] = fn.IdentityExp(newIndex)
			}
		}

		if len(replacements) > 0 {
			d.Dom = sortedDom
			d.f.DangerouslyReplaceExp(replacements)
		}
	}
	return d
}

func depEqual[IO DepIO](a, b Dep[IO]) bool {
	equal := slices.Equal(a.Dom, b.Dom)
	equal = equal && a.Codom == b.Codom
	equal = equal && fn.Equal(a.f, b.f)
	// The substitutions map is specifically not checked as "how" the fd got to its current state
	// does not affect the equality condition.

	return equal
}

type varFD = Dep[*engine.Variable]
type FD = Dep[engine.Attribute]
type varOrAttrFD = Dep[varOrAttr]

var (
	fdEqual          = depEqual[engine.Attribute]
	varFDEqual       = depEqual[*engine.Variable]
	varOrAttrFDEqual = depEqual[varOrAttr]
)

type varOrAttr struct {
	Var  *engine.Variable
	Attr *engine.Attribute
}

func (v varOrAttr) String() string {
	return fmt.Sprintf("varOrAttr(var: %v, attr: %v)", v.Var, v.Attr)
}

func Analyze(s *engine.State) {

}

func FDs(s *engine.State) map[*engine.Relation]*SetFunc[FD] {
	fds := map[*engine.Relation]*SetFunc[FD]{}
	oldFDs := maps.Clone(fds)
	first := true

	for first || !maps.EqualFunc(fds, oldFDs, func(a, b *SetFunc[FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			if _, ok := fds[head]; !ok {
				fds[head] = &SetFunc[FD]{equal: fdEqual}
			}
			fds[head].Union(HeadFDs(rl, fds))
		}
	}

	first = true
	for first || !maps.EqualFunc(fds, oldFDs, func(a, b *SetFunc[FD]) bool { return a.Equal(b) }) {
		first = false
		maps.Clear(oldFDs)
		for rel, s := range fds {
			oldFDs[rel] = s.Clone()
		}

		for _, rl := range s.Rules() {
			head := rl.Head()
			fdsNoR := maps.Clone(fds) // Note that this is a shallow copy
			fdsNoR[head] = &SetFunc[FD]{equal: fdEqual}

			fds[head].Intersect(HeadFDs(rl, fdsNoR))
		}
	}

	finalFDs := map[*engine.Relation]*SetFunc[FD]{}
	for rel, deps := range fds {
		finalFDs[rel] = &SetFunc[FD]{equal: fdEqual}
		for _, fd := range deps.Elems() {
			if !fd.Reflexive() {
				finalFDs[rel].Add(fd)
			}
		}
	}

	return finalFDs
}

func HeadFDs(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[FD]) *SetFunc[FD] {
	rDeps := &SetFunc[FD]{equal: fdEqual}
	rAttrs := Set[engine.Attribute]{}
	rAttrs.Add(rl.Head().Attrs()...)

	for _, fd := range DepsClosure(rl, existingFDs, false).Elems() {
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

func DepsClosure(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[FD], includeNeg bool) *SetFunc[FD] {
	varDeps := Deps(rl, existingFDs, includeNeg)
	newDeps := &SetFunc[varFD]{equal: varFDEqual}
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

	attrDeps := &SetFunc[FD]{equal: fdEqual}
	for _, vfd := range newDeps.Elems() {
		vafds := &SetFunc[varOrAttrFD]{equal: varOrAttrFDEqual}
		f := varOrAttrFD{
			Dom:   make([]varOrAttr, len(vfd.Dom)),
			Codom: varOrAttr{Var: vfd.Codom},
			f:     vfd.f,
		}
		for i, v := range vfd.Dom {
			f.Dom[i] = varOrAttr{Var: v}
		}
		vafds.Add(f)

		for _, v := range vfd.Dom {
			newVafds := &SetFunc[varOrAttrFD]{equal: varOrAttrFDEqual}
			for _, vafd := range vafds.Elems() {
				for _, a := range v.Attrs() {
					g := vafd.Clone()
					g = attrDomSub(g, a, v)
					newVafds.Add(g)
				}
			}
			vafds = newVafds
		}

		v := vfd.Codom
		newVafds := &SetFunc[varOrAttrFD]{equal: varOrAttrFDEqual}
		for _, vafd := range vafds.Elems() {
			for _, a := range v.Attrs() {
				g := vafd.Clone()
				g = attrCodomSub(g, a, v)
				newVafds.Add(g)
			}
		}
		vafds = newVafds

		for _, vafd := range vafds.Elems() {
			attrDeps.Add(varOrAttrFDToFD(vafd).Normalize())
		}
	}

	return attrDeps
}

func Deps(rl *engine.Rule, existingFDs map[*engine.Relation]*SetFunc[FD], includeNeg bool) *SetFunc[varFD] {
	basicDeps := &SetFunc[FD]{equal: fdEqual}

	relations := rl.Body()
	relations = append(relations, rl.Head())
	for _, rel := range relations {
		if !includeNeg && rl.IsNegated(rel) {
			continue
		}

		coreFDs := rel.CoreFDs()
		if len(coreFDs) > 0 {
			for _, coreFD := range coreFDs {
				basicDeps.Add(FD{
					Dom:   slices.Clone(coreFD.Dom),
					Codom: coreFD.Codom,
					f:     coreFD.Func.Clone(),
				})
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

			fd := FD{
				Dom:   []engine.Attribute{a},
				Codom: a,
				f:     fn.Identity(),
			}
			basicDeps.Add(fd)
		}
	}

	varDeps := &SetFunc[varFD]{equal: varFDEqual}
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
				g = varSub(g, v, a)
			} else {
				c, ok := rl.ConstOfAttr(a)
				if !ok {
					panic(fmt.Sprintf("Found an attribute %q which was not a variable or a constant", a))
				}
				g = constSub(g, c, a)
			}
		}

		varDeps.Add(varOrAttrFDToVarFD(g))
	}

	return varDeps
}

func funcSub[IO DepIO](sub Dep[IO], dep Dep[IO]) Dep[IO] {
	newDep := dep.Clone()
	if _, ok := newDep.substitutions[sub.Codom]; ok {
		//panic("The same input cannot be substituted twice for a given FD.")
		return newDep
	}
	newDep.Dom = nil
	newDep.substitutions[sub.Codom] = sub

	// If an input has already been substituted for dep, substitute it into any future
	// substitutions.
	subVars := Set[IO]{}
	subVars.Add(sub.Dom...)
	for _, transform := range newDep.substitutions {
		if subVars[transform.Codom] {
			sub = funcSub(transform, sub)
		}
	}

	subVars = Set[IO]{}
	subVars.Add(sub.Dom...)
	subDomIndices := make([]int, 0, len(sub.Dom))
	for i, v := range dep.Dom {
		if subVars[v] {
			subDomIndices = append(subDomIndices, i)
			// An input will only appear once in the domain
			subVars.Delete(v)
		}
	}

	newVars := make([]IO, 0, len(subVars.Elems()))
	for i, v := range sub.Dom {
		if subVars[v] {
			subDomIndices = append(subDomIndices, len(dep.Dom)+i)
			newVars = append(newVars, v)
		}
	}

	found := false
	for i, v := range dep.Dom {
		// This will only be run ONCE as an input can only appear one time in the domain of a given fd.
		if v == sub.Codom {
			if found {
				panic("The same input cannot appear multiple times in the same dependency")
			}
			found = true

			newDep.f.AddToDomain(len(newVars))
			newDep.f.SubstituteFunc(i, subDomIndices, sub.f)
		} else {
			newDep.Dom = append(newDep.Dom, v)
		}
	}

	// Only modify the function if a substitution match was found
	if found {
		newDep.Dom = append(newDep.Dom, newVars...)
	}

	return newDep
}

func varSub(vafd varOrAttrFD, v *engine.Variable, a engine.Attribute) varOrAttrFD {
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

	return vafd
}

func constSub(vafd varOrAttrFD, val int, a engine.Attribute) varOrAttrFD {
	for i := 0; i < len(vafd.Dom); i++ {
		if vafd.Dom[i].Attr != nil && *vafd.Dom[i].Attr == a {
			vafd.f.SubstituteFunc(i, []int{}, fn.Const(val))
			vafd.Dom = slices.Delete(vafd.Dom, i, i+1)
			i--
		}
	}
	return vafd
}

func attrDomSub(vafd varOrAttrFD, a engine.Attribute, v *engine.Variable) varOrAttrFD {
	for i := range vafd.Dom {
		if vafd.Dom[i].Var != nil && vafd.Dom[i].Var == v {
			vafd.Dom[i].Attr = &a
			vafd.Dom[i].Var = nil
		}
	}
	return vafd
}

func attrCodomSub(vafd varOrAttrFD, a engine.Attribute, v *engine.Variable) varOrAttrFD {
	if vafd.Codom.Var != nil && vafd.Codom.Var == v {
		vafd.Codom.Attr = &a
		vafd.Codom.Var = nil
	}
	return vafd
}

func varOrAttrFDToVarFD(vafd varOrAttrFD) varFD {
	vfd := varFD{
		Dom:           make([]*engine.Variable, len(vafd.Dom)),
		f:             vafd.f,
		substitutions: map[*engine.Variable]varFD{},
	}
	for i, va := range vafd.Dom {
		if va.Var == nil {
			panic(fmt.Sprintf("All attributes should have been replaced by variables in the second phase of Deps(); %q was not", va.Attr.String()))
		}
		vfd.Dom[i] = va.Var
	}

	if vafd.Codom.Var == nil {
		panic(fmt.Sprintf("All attributes should have been replaced by variables in the second phase of Deps(); %q was not", vafd.Codom.Attr.String()))
	}
	vfd.Codom = vafd.Codom.Var

	return vfd
}

func varOrAttrFDToFD(vafd varOrAttrFD) FD {
	fd := FD{
		Dom: make([]engine.Attribute, len(vafd.Dom)),
		f:   vafd.f,
	}
	for i, va := range vafd.Dom {
		if va.Attr == nil {
			panic("All variables should have been replaced by attributes in the second phase of DepsClosure()")
		}
		fd.Dom[i] = *va.Attr
	}

	if vafd.Codom.Attr == nil {
		panic("All variables should have been replaced by attributes in the second phase of DepsClosure()")
	}
	fd.Codom = *vafd.Codom.Attr

	return fd
}
