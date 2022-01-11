package engine

func (r *relation) empty() bool {
	return len(r.indexes[0]) == 0
}

func (r *relation) push(d []string, loc string, time int) bool {
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

	return true
}

func (r *relation) contains(d []string, loc string, time int) bool {
	if len(r.indexes) != len(d) {
		return false
	}

	var factSet []*fact
	var ok bool
	if factSet, ok = r.lookup(0, d[0], loc, time); !ok {
		return false
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

func (r *relation) lookup(attrIndex int, attrVal string, loc string, time int) ([]*fact, bool) {
	lt := locTime{}
	if !r.readOnly {
		lt = locTime{loc, time}
	}

	matched, ok := r.indexes[attrIndex][attrVal][lt]
	return matched, ok
}

// TODO: Replace this with an iterator-like variant for efficiency
func (r *relation) all(loc string, time int) []*fact {
	var facts []*fact
	for attr := range r.indexes[0] {
		// A match is guaranteed to be found
		matched, _ := r.lookup(0, attr, loc, time)
		facts = append(facts, matched...)
	}

	return facts
}

// TODO: Replace this with an iterator-like variant for efficiency
func (r *relation) allAcrossSpaceTime() []*fact {
	var facts []*fact
	for _, factSuperSet := range r.indexes[0] {
		for _, factSet := range factSuperSet {
			facts = append(facts, factSet...)
		}
	}

	return facts
}
