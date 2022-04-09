package deps

import (
	"strings"

	"github.com/rithvikp/dedalus/analysis/fn"
	"github.com/rithvikp/dedalus/engine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type PartitionFunc = Dep[engine.Attribute] // TODO: This is a temporary bypass
var (
	partitionFuncEqual = depEqual[engine.Attribute]
)

type DistFunction struct {
	Dom []engine.Attribute
	f   fn.Func
}

func (f DistFunction) String() string {
	b := strings.Builder{}
	b.WriteString("{ Dom: [")
	for i, v := range f.Dom {
		b.WriteString(v.String())
		if i < len(f.Dom)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("], Func: ")
	b.WriteString(f.f.String())
	b.WriteString(" }")

	return b.String()
}

func DistFunctionEqual(a, b DistFunction) bool {
	equal := slices.Equal(a.Dom, b.Dom)
	equal = equal && fn.Equal(a.f, b.f)
	return equal
}

type DistPolicy map[*engine.Relation]DistFunction

func DistPolicyEqual(a, b DistPolicy) bool {
	return maps.EqualFunc(a, b, DistFunctionEqual)
}

type distPolicy map[*engine.Relation]PartitionFunc

func (p distPolicy) Clone() distPolicy {
	newP := distPolicy{}
	for k, v := range p {
		newP[k] = v.Clone()
	}
	return newP
}

func distPolicyEqual(a, b distPolicy) bool {
	return maps.EqualFunc(a, b, partitionFuncEqual)
}

// TODO: Things to check:
// - Skipping relations which appear in the head
// - Only looking at the body for shared rules

func DistibutionPolicies(s *engine.State) []DistPolicy {
	copartDeps := CDs(s)
	policies := SetFunc[distPolicy]{equal: distPolicyEqual}
	for _, rel := range s.NonEDBRelations() {
		// Skip any relations which only appear in the head
		if !rel.AppearsInABody() {
			continue
		}
		for _, a := range rel.Attrs() {
			policies.Add(distPolicy{rel: modOnAttr(a)})
		}
	}

	i := 0
	addedPolicy := false
	for {
		if addedPolicy {
			i = 0
		}
		addedPolicy = false
		if i >= policies.Len() {
			break
		}
		policy := policies.Elems()[i]

		// Populate rWithNoPartFunc with all relations that do not have partition functions in this
		// policy but appear in the same rule as some relation in this policy.
		rWithNoPartFunc := Set[*engine.Relation]{}
		for _, rel := range s.NonEDBRelations() {
			if !rel.AppearsInABody() {
				continue
			}
			if _, ok := policy[rel]; ok {
				continue
			}

			sharesRule := false
			// TODO: This is quite inefficient (due to the "haveSharedRules" call)
			for copartRel := range policy {
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
			i++
			continue
		}

		// See if all relations are compatible with this policy
		relToCheck := maps.Keys(rWithNoPartFunc)[0]
		relToCheckDom := Set[engine.Attribute]{}
		relToCheckDom.Add(relToCheck.Attrs()...)

		partFuncsWithRDom := map[*engine.Relation]*SetFunc[PartitionFunc]{}
		for copartRel := range policy {
			// r = relToCheck, s = copartRel
			if !haveSharedRules(relToCheck, copartRel) {
				continue
			}

			partFuncs := &SetFunc[PartitionFunc]{equal: partitionFuncEqual}
			partFuncs.Add(policy[copartRel])
			oldPartFuncs := &SetFunc[PartitionFunc]{equal: partitionFuncEqual}

			for !partFuncs.Equal(oldPartFuncs) {
				oldPartFuncs = partFuncs.Clone()
				for _, g := range oldPartFuncs.Elems() {
					for _, h := range copartDeps[CDMap{Dom: relToCheck, Codom: copartRel}].Elems() {
						partFuncs.Add(funcSub(h, g))
					}
				}
			}
			partFuncsWithRDom[copartRel] = &SetFunc[PartitionFunc]{equal: partitionFuncEqual}
			for _, f := range partFuncs.Elems() {
				for _, a := range f.Dom {
					if relToCheckDom[a] {
						partFuncsWithRDom[copartRel].Add(f)
					}
				}
			}

			if partFuncsWithRDom[copartRel].Len() == 0 {
				policies.Delete(policy)
				// Do not increment i as the current policy was removed
				// TODO: This currently assumes that SetFunc has a fixed order, which is not
				// an assumption that should be made.
				continue
			}
		}

		consistentPartFuncs := &SetFunc[PartitionFunc]{equal: partitionFuncEqual}
		for _, funcs := range partFuncsWithRDom {
			consistentPartFuncs.Union(funcs)
		}
		policies.Delete(policy)

		for _, relToCheckPartFunc := range consistentPartFuncs.Elems() {
			newDistPolicy := policy.Clone()
			newDistPolicy[relToCheck] = relToCheckPartFunc
			policies.Add(newDistPolicy)
			addedPolicy = true
		}

		// Do not increment as the current policy was removed
	}

	finalPolicies := make([]DistPolicy, 0, policies.Len())
	for _, p := range policies.Elems() {
		finalP := DistPolicy{}
		for rel, pf := range p {
			pf = pf.Normalize()
			finalP[rel] = DistFunction{
				Dom: pf.Dom,
				f:   pf.f,
			}
		}
		finalPolicies = append(finalPolicies, finalP)
	}
	return finalPolicies
}

func haveSharedRules(r1, r2 *engine.Relation) bool {
	for _, rl := range r1.Rules() {
		if rl.Head() == r1 {
			continue
		}
		if slices.Contains(rl.Body(), r2) /*|| rl.Head() == r2*/ {
			return true
		}
	}
	return false
}

func modOnAttr(a engine.Attribute) PartitionFunc {
	return PartitionFunc{
		Dom: []engine.Attribute{a},
		f:   fn.Identity(),
	}
}
