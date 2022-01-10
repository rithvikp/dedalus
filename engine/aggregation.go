package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// aggregator defines various supported aggregation functions.
//
// To add a new aggregator, add a new definition to the const block, add this new aggregator to the
// case statement in Valid(), and add an implementation to Do().
type aggregator string

const (
	aggregatorCount aggregator = "count"
	aggregatorMax   aggregator = "max"
	aggregatorMin   aggregator = "min"
	aggregatorSum   aggregator = "sum"
)

func (a aggregator) Valid() bool {
	switch a {
	case aggregatorCount, aggregatorMax, aggregatorMin, aggregatorSum:
		return true
	default:
		return false
	}
}

// It is (for now) assumed that the strings can be converted to the correct type for the aggregation
// operation (only numbers for now). While this is not great, until a better type system is implemented,
// it will suffice.
func (a aggregator) Do(prev string, val string) string {
	noPrev := false
	if prev == "" {
		noPrev = true
		prev = "0"
	}

	float := false
	var pi int
	var pf float64
	var vi int
	var vf float64
	var err error

	pi, err = strconv.Atoi(prev)
	if err != nil {
		float = true
	}
	pf, err = strconv.ParseFloat(prev, 64)
	if err != nil {
		panic(err)
	}

	vi, err = strconv.Atoi(val)
	if err != nil {
		float = true
	}
	vf, err = strconv.ParseFloat(val, 64)
	if err != nil {
		panic(err)
	}

	switch a {
	case aggregatorCount:
		if float {
			panic("the count aggregator was passed a float, but it only works with ints")
		}

		return strconv.Itoa(pi + 1)

	case aggregatorMax, aggregatorMin:
		var next string
		if !float {
			nextI := pi
			if noPrev || a == aggregatorMax && vi > pi || a == aggregatorMin && vi < pi {
				nextI = vi
			}
			next = strconv.Itoa(nextI)
		} else {
			nextF := pf
			if noPrev || a == aggregatorMax && vf > pf || a == aggregatorMin && vf < pf {
				nextF = vf
			}
			next = fmt.Sprintf("%f", nextF)
		}
		return next

	case aggregatorSum:
		var next string
		if !float {
			next = strconv.Itoa(pi + vi)
		} else {
			next = fmt.Sprintf("%f", pf+vf)
		}
		return next
	}

	return ""
}

// This function operates on the output of join.
// This is EXTREMELY hacky/non-clean code, but it works as a proof of concept.
func aggregate(rl *rule, data [][]string) [][]string {
	type aggIndex struct {
		i   int
		agg *aggregator
	}

	var nonAggIndices []int
	var aggIndices []aggIndex
	for i, t := range rl.headVarMapping {
		if t.agg == nil {
			nonAggIndices = append(nonAggIndices, i)
		} else {
			ai := aggIndex{i: i, agg: t.agg}
			aggIndices = append(aggIndices, ai)
		}
	}
	nonAggIndices = append(nonAggIndices, len(rl.headVarMapping))

	type aggVar struct {
		i   int
		val string
	}
	type pendingAgg struct {
		nonAgg []string
		agg    []*aggVar
	}

	pendingAggData := map[string]*pendingAgg{}
	for _, d := range data {
		var nonAgg []string
		for _, i := range nonAggIndices {
			nonAgg = append(nonAgg, d[i])
		}

		b, _ := json.Marshal(nonAgg)
		if pa, ok := pendingAggData[string(b)]; !ok {
			pa := pendingAgg{nonAgg: nonAgg}
			for _, ai := range aggIndices {
				av := aggVar{i: ai.i, val: ai.agg.Do("", d[ai.i])}
				pa.agg = append(pa.agg, &av)
			}
			pendingAggData[string(b)] = &pa
		} else {
			for i, ai := range aggIndices {
				av := pa.agg[i]
				av.val = ai.agg.Do(av.val, d[ai.i])
			}
		}
	}

	var aggData [][]string
	for _, pa := range pendingAggData {
		d := make([]string, len(rl.headVarMapping)+1)
		for i, di := range nonAggIndices {
			d[di] = pa.nonAgg[i]
		}
		for _, av := range pa.agg {
			d[av.i] = av.val
		}
		aggData = append(aggData, d)
	}

	return aggData
}
