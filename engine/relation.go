package engine

func (r *relation) push(d []string, loc string, time int) bool {
	if r.contains(d, loc, time) {
		return false
	}

	f := &fact{
		data:      d,
		location:  loc,
		timestamp: time,
	}

	lt := locTime{location: loc, timestamp: time}

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

	lt := locTime{location: loc, timestamp: time}
	var factSet []*fact
	var ok bool
	if factSet, ok = r.indexes[0][d[0]][lt]; !ok {
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

// TODO: Replace this with an iterator-like variant for efficiency
func (r *relation) all(loc string, time int) []*fact {
	var facts []*fact
	lt := locTime{loc, time}
	for _, factSuperSet := range r.indexes[0] {
		for _, f := range factSuperSet[lt] {
			facts = append(facts, f)
		}
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
