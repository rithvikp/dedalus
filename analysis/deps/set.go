package deps

import (
	"fmt"

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

func (s *SetFunc[K]) Delete(elem K) {
	i := slices.IndexFunc(s.elems, func(e K) bool { return s.equal(e, elem) })
	if i == -1 {
		return
	}

	s.elems = slices.Delete(s.elems, i, i+1)
}

func (s *SetFunc[K]) Clone() *SetFunc[K] {
	c := &SetFunc[K]{equal: s.equal}
	c.Union(s)
	return c
}

func (s *SetFunc[K]) Elems() []K {
	return s.elems
}

func (s *SetFunc[K]) Len() int {
	return len(s.elems)
}

func (s *SetFunc[K]) Equal(other *SetFunc[K]) bool {
	if s.Len() != other.Len() {
		return false
	}
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

func (s *SetFunc[K]) String() string {
	elems := make([]any, len(s.elems))
	for i, e := range s.elems {
		elems[i] = e
	}
	return fmt.Sprint(elems...)
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
