package deps

import (
	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type PartitionFunction struct {
	attribute engine.Attribute
	f         Function
	modulo    int
}

func partitionFunctionEqual(a, b PartitionFunction) bool {
	equal := a.attribute == b.attribute
	equal = equal && funcEqual(a.f, b.f)
	equal = equal && a.modulo == b.modulo
	return equal
}

type DistPolicy map[*engine.Relation]PartitionFunction

func distPolicyEqual(a, b DistPolicy) bool {
	return maps.EqualFunc(a, b, partitionFunctionEqual)
}

func DistibutionPolicies(s *engine.State) []*DistPolicy {
	copartDeps := CDs(s)
	distPolicies := SetFunc[DistPolicy]{equal: distPolicyEqual}
	for _, rel := range s.Relations() {
		for _, a := range rel.Attrs() {
			distPolicies.Add(DistPolicy{rel: modOnAttr(a)})
		}
	}

	for _, distPolicy := range distPolicies.Elems() {
		// Populate rWithNoPartFunc with all relations that do not have partition functions in this
		// policy but appear in the same rule as some relation in this policy.
		rWithNoPartFunc := Set[*engine.Relation]{}
		for _, rel := range s.Relations() {
			if _, ok := distPolicy[rel]; ok {
				continue
			}

			sharesRule := false
			// TODO: This is quite inefficient (due to the "haveSharedRules" call)
			for copartRel := range distPolicy {
				if haveSharedRules(rel, copartRel) {
					sharesRule = true
					break
				}
			}

			if sharesRule {
				rWithNoPartFunc.Add(rel)
			}
		}

		if len(rWithNoPartFunc) == 0 {
			continue
		}

		// See if all relations are compatible with this policy
		relToCheck := maps.Keys(rWithNoPartFunc)[0]
		partFuncsWithRDom := SetFunc[PartitionFunction]{equal: partitionFunctionEqual}
		for copartRel := range distPolicy {
			if !haveSharedRules(relToCheck, copartRel) {
				continue
			}

			partFuncs := SetFunc[DistPolicy]{equal: distPolicyEqual}
			for {
				oldPartFuncs := partFuncs.Clone()
				for _, g := range oldPartFuncs.Elems() {
					for _, h := range copartDeps[CDMap{Dom: relToCheck, Codom: copartRel}].Elems() {
						_ = g
						_ = h
						_ = partFuncsWithRDom
					}
				}
			}
		}
	}
	return nil
}

func haveSharedRules(r1, r2 *engine.Relation) bool {
	for _, rl := range r1.Rules() {
		if slices.Contains(rl.Body(), r2) || rl.Head() == r2 {
			return true
		}
	}
	return false
}

func modOnAttr(a engine.Attribute) PartitionFunction {
	return PartitionFunction{
		attribute: a,
		f:         IdentityFunc(),
	}
}
