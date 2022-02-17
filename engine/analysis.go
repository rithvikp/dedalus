package engine

type SubComponent struct {
	Rules            []*Rule
	IngressRelations []*Relation
	EgressRelations  []*Relation
}

// Start at a rule, move up and down until a location change is noticed.
func (s *State) SubComponents() []*SubComponent {
	seen := map[*Rule]bool{}
	var components []*SubComponent

	for _, origRl := range s.rules {
		if seen[origRl] {
			continue
		}

		c := SubComponent{}
		fringe := []*Rule{origRl}
		for len(fringe) > 0 {
			rl := fringe[0]
			fringe = fringe[1:]
			if seen[rl] {
				// TODO: Ensure the parent has been matched to the same component
				continue
			}

			seen[rl] = true
			c.Rules = append(c.Rules, rl)

			addToFringe := func(rl *Rule) bool {
				if seen[rl] {
					// TODO: Ensure the parent has been matched to the same component
					return false
				}
				fringe = append(fringe, rl)
				return true
			}

			for _, rel := range rl.body {
				for _, parent := range rel.headRules {
					if parent.headLocVar != parent.bodyLocVar {
						c.IngressRelations = append(c.IngressRelations, rel)
						continue
					} else if !addToFringe(parent) {
						continue
					}
				}
			}

			if rl.headLocVar == rl.bodyLocVar {
				for _, child := range rl.head.bodyRules {
					if !addToFringe(child) {
						continue
					}
				}
			} else {
				c.EgressRelations = append(c.EgressRelations, rl.head)
			}
		}
		components = append(components, &c)
	}

	return components
}
