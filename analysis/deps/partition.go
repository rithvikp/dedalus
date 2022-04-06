package deps

import (
	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

//type PartitionFunction struct {
//attribute engine.Attribute
//f         Function
//modulo    int
//}

//func partitionFunctionEqual(a, b PartitionFunction) bool {
//equal := a.attribute == b.attribute
//equal = equal && funcEqual(a.f, b.f)
//equal = equal && a.modulo == b.modulo
//return equal
//}

type PartitionFunc = Dep[engine.Attribute] // TODO: This is a temporary bypass
var (
	partitionFuncEqual = depEqual[engine.Attribute]
)

type DistPolicy map[*engine.Relation]*PartitionFunc

func (p DistPolicy) Clone() DistPolicy {
	newP := DistPolicy{}
	for k, v := range p {
		d := v.Clone()
		newP[k] = &d
	}
	return newP
}

func distPolicyEqual(a, b DistPolicy) bool {
	return maps.EqualFunc(a, b, partitionFuncEqual)
}

func DistibutionPolicies(s *engine.State) []DistPolicy {
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
		relToCheckDom := Set[engine.Attribute]{}
		relToCheckDom.Add(relToCheck.Attrs()...)

		partFuncsWithRDom := map[*engine.Relation]*SetFunc[*PartitionFunc]{}
		for copartRel := range distPolicy {
			// r = relToCheck, s = copartRel
			if !haveSharedRules(relToCheck, copartRel) {
				continue
			}

			partFuncs := &SetFunc[*PartitionFunc]{equal: partitionFuncEqual}
			partFuncs.Add(distPolicy[copartRel])
			oldPartFuncs := &SetFunc[*PartitionFunc]{equal: partitionFuncEqual}

			for !partFuncs.Equal(oldPartFuncs) {
				oldPartFuncs = partFuncs.Clone()
				for _, g := range oldPartFuncs.Elems() {
					for _, h := range copartDeps[CDMap{Dom: relToCheck, Codom: copartRel}].Elems() {
						partFuncs.Add(funcSub(h, g))
					}
				}
			}
			partFuncsWithRDom[copartRel] = &SetFunc[*PartitionFunc]{equal: partitionFuncEqual}
			for _, f := range partFuncs.Elems() {
				for _, a := range f.Dom {
					if relToCheckDom[a] {
						partFuncsWithRDom[copartRel].Add(f)
					}
				}
			}

			if partFuncsWithRDom[copartRel].Len() == 0 {
				distPolicies.Delete(distPolicy)
				continue
			}
		}

		consistentPartFuncs := &SetFunc[*PartitionFunc]{equal: partitionFuncEqual}
		for _, funcs := range partFuncsWithRDom {
			consistentPartFuncs.Union(funcs)
		}
		distPolicies.Delete(distPolicy)

		for _, relToCheckPartFunc := range consistentPartFuncs.Elems() {
			newDistPolicy := distPolicy.Clone()
			newDistPolicy[relToCheck] = relToCheckPartFunc
			distPolicies.Add(newDistPolicy)
		}
	}

	return distPolicies.Elems()
}

func haveSharedRules(r1, r2 *engine.Relation) bool {
	for _, rl := range r1.Rules() {
		if slices.Contains(rl.Body(), r2) || rl.Head() == r2 {
			return true
		}
	}
	return false
}

func modOnAttr(a engine.Attribute) *PartitionFunc {
	return &PartitionFunc{
		Dom: []engine.Attribute{a},
		f:   IdentityFunc(),
	}
}
