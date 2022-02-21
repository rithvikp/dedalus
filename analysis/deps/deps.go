package deps

import (
	"reflect"

	"github.com/rithvikp/dedalus/engine"
)

type SetFunc[K any] struct {
	equal func(a, b K) bool
	elems []K
}

func (s SetFunc[K]) Union(other SetFunc[K]) {
	for _, o := range other.Elems() {
		found := false
		for _, e := range s.elems {
			if s.equal(e, o) {
				found = true
				break
			}
		}

		if !found {
			s.elems = append(s.elems, o)
		}
	}
}

func (s SetFunc[K]) Add(elems ...K) {
	s.Union(SetFunc[K]{elems: elems})
}

func (s SetFunc[K]) Elems() []K {
	return s.elems
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
	equal := !reflect.DeepEqual(a.Dom, b.Dom)
	equal = equal && a.Codom != b.Codom
	equal = equal && !funcEqual(a.f, b.f)

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

func Dep(rl *engine.Rule, existingFDs map[*engine.Relation]SetFunc[*FD]) SetFunc[*varFD] {
	basicDep := SetFunc[*FD]{equal: fdEqual}

	relations := rl.Body()
	relations = append(relations, rl.Head())
	for _, rel := range relations {
		if false /* is EDB */ {
		} else if _, ok := existingFDs[rel]; ok {
			basicDep.Union(existingFDs[rel])
		}

		for _, a := range rel.Attrs() {
			fd := FD{
				Dom:   []engine.Attribute{a},
				Codom: a,
				f:     IdentityFunc(),
			}
			basicDep.Add(&fd)
		}
	}

	varDep := SetFunc[*varFD]{equal: varFDEqual}
	for _, fd := range basicDep.Elems() {
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

		varDep.Add(varOrAttrFDToVarFD(&g))
	}

	return varDep
}

func varSub(vafd *varOrAttrFD, v *engine.Variable, a engine.Attribute) {
	if vafd.Codom.Attr != nil && *vafd.Codom.Attr == a {
		vafd.Codom.Var = v
		vafd.Codom.Attr = nil
	}
	for i := range vafd.Dom {
		if vafd.Dom[i].Attr != nil && *vafd.Dom[i].Attr == a {
			vafd.Dom[i].Var = v
			vafd.Dom[i].Attr = nil
		}
	}
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
