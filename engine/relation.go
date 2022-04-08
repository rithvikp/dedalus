package engine

import (
	"fmt"

	"golang.org/x/exp/slices"
)

type Relation struct {
	id          string
	readOnly    bool
	autoPersist bool // Pascal-Cased relations are automatically persisted

	headRules []*Rule
	bodyRules []*Rule
	indexes   []map[string]map[locTime][]*fact
	ltIndex   map[locTime]struct{}
}

type Variable struct {
	id       string
	attrs    map[string][]Attribute
	constant bool // Represents a constant term
}

func (v Variable) String() string {
	return v.id
}

type Attribute struct {
	relation *Relation
	index    int
}

func (a Attribute) String() string {
	id := "<nil>"
	if a.relation != nil {
		id = a.relation.id
	}
	return fmt.Sprintf("Attr(%s,%d)", id, a.index)
}

func (a Attribute) LessThan(b Attribute) bool {
	if a.relation.id < b.relation.id {
		return true
	} else if a.relation.id > b.relation.id {
		return false
	}
	return a.index < b.index
}

type fact struct {
	data      []string
	location  string
	timestamp int
}

type locTime struct {
	location  string
	timestamp int
}

func (r *Relation) Attrs() []Attribute {
	attrs := make([]Attribute, r.numAttrs())
	for i := 0; i < len(attrs); i++ {
		attrs[i] = Attribute{relation: r, index: i}
	}
	return attrs
}

func (r *Relation) String() string {
	return r.id
}

func (r *Relation) ID() string {
	return r.id
}

func (r *Relation) IsEDB() bool {
	return r.readOnly // FIXME
}

func (r *Relation) AppearsInBody() bool {
	return len(r.bodyRules) > 0
}

func (r *Relation) Rules() []*Rule {
	return append(slices.Clone(r.bodyRules), r.headRules...)
}

func newRelation(id string, readOnly, autoPersist bool, indexCount int) *Relation {
	r := &Relation{
		id:          id,
		readOnly:    readOnly,
		autoPersist: autoPersist,
		indexes:     make([]map[string]map[locTime][]*fact, indexCount),
		ltIndex:     map[locTime]struct{}{},
	}

	for i := range r.indexes {
		r.indexes[i] = map[string]map[locTime][]*fact{}
	}
	return r
}

func (r *Relation) numAttrs() int {
	return len(r.indexes)
}

func (r *Relation) push(d []string, loc string, time int) bool {
	if r.contains(d, loc, time) {
		return false
	}

	lt := locTime{}
	if !r.readOnly {
		lt = locTime{loc, time}
	}

	f := &fact{
		data:      d,
		location:  lt.location,
		timestamp: lt.timestamp,
	}

	for i := range d {
		if _, ok := r.indexes[i][d[i]]; !ok {
			r.indexes[i][d[i]] = map[locTime][]*fact{}
		}
		r.indexes[i][d[i]][lt] = append(r.indexes[i][d[i]][lt], f)
	}
	r.ltIndex[lt] = struct{}{}

	return true
}

func (r *Relation) contains(d []string, loc string, time int) bool {
	if len(r.indexes) != len(d) {
		return false
	}

	var factSet []*fact
	var ok bool
	if len(r.indexes) > 0 {
		if factSet, ok = r.lookup(0, d[0], loc, time); !ok {
			return false
		}
	} else {
		if factSet, ok = r.lookup(-1, "", loc, time); !ok {
			return false
		}
	}

	for _, f := range factSet {
		found := true
		for i := range d {
			if f.data[i] != d[i] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}

	return false
}

func (r *Relation) lookup(attrIndex int, attrVal string, loc string, time int) ([]*fact, bool) {
	lt := locTime{}
	if !r.readOnly {
		lt = locTime{loc, time}
	}

	if len(r.indexes) == 0 {
		_, ok := r.ltIndex[lt]
		if !ok {
			return nil, false
		}
		return []*fact{{location: lt.location, timestamp: lt.timestamp}}, true
	}

	matched, ok := r.indexes[attrIndex][attrVal][lt]
	return matched, ok
}

// TODO: Replace this with an iterator-like variant for efficiency
func (r *Relation) all(loc string, time int) []*fact {
	var facts []*fact
	if len(r.indexes) == 0 {
		_, ok := r.lookup(-1, "", loc, time)
		if ok {
			facts = append(facts, &fact{location: loc, timestamp: time})
		}
	} else {
		for attr := range r.indexes[0] {
			// A match is guaranteed to be found
			matched, _ := r.lookup(0, attr, loc, time)
			facts = append(facts, matched...)
		}
	}

	return facts
}

// TODO: Replace this with an iterator-like variant for efficiency
func (r *Relation) allAcrossSpaceTime() []*fact {
	var facts []*fact
	if len(r.indexes) == 0 {
		for lt := range r.ltIndex {
			facts = append(facts, &fact{location: lt.location, timestamp: lt.timestamp})
		}
	} else {
		for _, factSuperSet := range r.indexes[0] {
			for _, factSet := range factSuperSet {
				facts = append(facts, factSet...)
			}
		}
	}

	return facts
}
