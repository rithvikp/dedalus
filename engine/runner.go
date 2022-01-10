package engine

import (
	"crypto/sha1"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/rithvikp/dedalus/ast"
)

// TODO: underscore identifiers, validate for safe negation, conditions, user-defined r.r.t.'s,
//	     auto-persisted relations

type SemanticError struct {
	Position lexer.Position
	Message  string
}

func (e *SemanticError) Error() string {
	return fmt.Sprintf("semantic error at %s: %s", e.Position.String(), e.Message)
}

func newSemanticError(msg string, pos lexer.Position) *SemanticError {
	return &SemanticError{Position: pos, Message: msg}
}

type timeModel int

const (
	timeModelSame = iota
	timeModelSuccessor
	timeModelAsync
)

const (
	successorRelationName = "succ"
	chooseRelationName    = "choose"
)

var lateHandleAtoms = map[string]struct{}{
	successorRelationName: {},
	chooseRelationName:    {},
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

type relation struct {
	id        string
	bodyRules []*rule

	indexes []map[string]map[locTime][]*fact
}

type attribute struct {
	relation *relation
	index    int
}

type variable struct {
	id    string
	attrs map[string][]*attribute
}

type condition struct {
	v1 *variable
	v2 *variable
	op string
}

type headTerm struct {
	agg *aggregator // Optional
	v   *variable
}

type rule struct {
	id          string
	head        *relation
	body        []*relation
	negatedBody []*relation
	conditions  []condition

	// The index in the head relation mapped to the corresponding variable in the body.
	headVarMapping []headTerm

	// The keys are relations and the list of variables corresponds to the variables in the atom.
	vars map[string][]*variable

	timeModel   timeModel
	bodyLocVar  *variable
	headLocVar  *variable
	bodyTimeVar *variable
	headTimeVar *variable

	hasAggregation bool
}

type Runner struct {
	relations map[string]*relation

	rules []*rule

	currentTimestamp int
	locations        map[string]struct{}
}

// stringToNumber converts the given string to an integer (if possible) and a float. If the returned
// boolean is true, only the float representation is valid.
func stringToNumber(s string) (int, float64, bool, error) {
	float := false
	i, err := strconv.Atoi(s)
	if err != nil {
		float = true
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, 0, false, err
	}

	return i, f, float, nil
}

func (f *fact) equals(other *fact) bool {
	if len(f.data) != len(other.data) {
		return false
	} else if f.location != other.location {
		return false
	} else if f.timestamp != other.timestamp {
		return false
	}

	for i := range f.data {
		if f.data[i] != other.data[i] {
			return false
		}
	}
	return true
}

func (c condition) Eval(val1, val2 string) bool {
	switch c.op {
	case "=":
		return val1 == val2
	case "!=":
		return val1 != val2
	}

	v1i, v1f, float1, err := stringToNumber(val1)
	if err != nil {
		panic(err)
	}
	v2i, v2f, float2, err := stringToNumber(val2)
	if err != nil {
		panic(err)
	}
	float := float1 || float2

	switch c.op {
	case ">":
		if !float {
			return v1i > v2i
		}
		return v1f > v2f
	case ">=":
		if !float {
			return v1i >= v2i
		}
		return v1f >= v2f
	case "<":
		if !float {
			return v1i < v2i
		}
		return v1f < v2f
	case "<=":
		if !float {
			return v1i <= v2i
		}
		return v1f <= v2f
	}

	return false
}

func NewRunner(p *ast.Program) (*Runner, error) {
	runner := Runner{
		relations: map[string]*relation{},
		locations: map[string]struct{}{},
	}

	addRel := func(id string, vars int, pos lexer.Position, head bool, rl *rule) (*relation, error) {
		var ok bool
		var r *relation
		if r, ok = runner.relations[id]; !ok {
			r = &relation{
				id:      id,
				indexes: make([]map[string]map[locTime][]*fact, vars-2),
			}
			for i := range r.indexes {
				r.indexes[i] = map[string]map[locTime][]*fact{}
			}
			runner.relations[id] = r
			if !head {
				r.bodyRules = append(r.bodyRules, rl)
			}
		} else {
			if len(r.indexes) != vars-2 {
				return nil, newSemanticError("the number of attributes must be constant for any given relation", pos)
			}
			if !head {
				r.bodyRules = append(r.bodyRules, rl)
			}
		}

		return r, nil
	}

	for i, astStatement := range p.Statements {
		if astStatement.Rule != nil {
			astRule := astStatement.Rule
			r := &rule{
				id:             strconv.Itoa(i),
				headVarMapping: make([]headTerm, len(astRule.Head.Terms)-2),
			}
			vars := map[string]*variable{}
			r.vars = map[string][]*variable{}

			astHeadVars := astRule.Head.Terms
			if len(astHeadVars) < 2 {
				return nil, newSemanticError("all non-replicated read-only relations must have time and location attributes", astRule.Head.Pos)
			}

			var err error
			r.head, err = addRel(astRule.Head.Name, len(astRule.Head.Terms), astRule.Pos, true, r)
			if err != nil {
				return nil, err
			}
			r.headLocVar = &variable{
				id:    astHeadVars[len(astHeadVars)-2].Variable.Name,
				attrs: map[string][]*attribute{},
			}
			vars[r.headLocVar.id] = r.headLocVar
			r.headTimeVar = &variable{
				id:    astHeadVars[len(astHeadVars)-1].Variable.Name,
				attrs: map[string][]*attribute{},
			}
			vars[r.headTimeVar.id] = r.headTimeVar

			runner.rules = append(runner.rules, r)

			headVars := map[string][]int{}
			aggregatedIndices := map[int]aggregator{}
			for j, v := range astHeadVars {
				if j >= len(astHeadVars)-2 {
					break
				}

				headVars[v.Variable.Name] = append(headVars[v.Variable.Name], j)
				if v.Aggregator == nil {
					continue
				}

				r.hasAggregation = true
				agg := aggregator(*v.Aggregator)
				if !agg.Valid() {
					return nil, newSemanticError(fmt.Sprintf("invalid aggregation function %q", agg), v.Pos)
				}
				aggregatedIndices[j] = agg
			}

			var lateAtoms []*ast.Atom
			var conditions []*ast.Condition
			for _, astTerm := range astRule.Body {
				if astTerm.Condition != nil {
					conditions = append(conditions, astTerm.Condition)
					continue
				}

				astAtom := astTerm.Atom
				if _, ok := lateHandleAtoms[astAtom.Name]; ok {
					lateAtoms = append(lateAtoms, astAtom)
					continue
				}

				// TODO: Cleanup time/loc parsing (there are many "-2"'s when looking at # of variables
				// due to this issue).
				if len(astAtom.Variables) < 2 {
					return nil, newSemanticError("all non-replicated read-only relations must have time and location attributes", astRule.Head.Pos)
				}

				rel, err := addRel(astAtom.Name, len(astAtom.Variables), astAtom.Pos, false, r)
				if err != nil {
					return nil, err
				}

				if astAtom.Negated {
					r.negatedBody = append(r.negatedBody, rel)
				} else {
					r.body = append(r.body, rel)
				}
				r.vars[astAtom.Name] = make([]*variable, len(astAtom.Variables)-2)

				addToHeadVarMapping := func(v *variable) {
					if indices, ok := headVars[v.id]; ok {
						for _, k := range indices {
							hv := headTerm{v: v}
							if agg, ok := aggregatedIndices[k]; ok {
								hv.agg = &agg
							}
							r.headVarMapping[k] = hv
						}
					}
				}

				atomLoc := astAtom.Variables[len(astAtom.Variables)-2].Name
				if r.bodyLocVar == nil {
					if atomLoc != r.headLocVar.id {
						r.bodyLocVar = &variable{id: atomLoc, attrs: map[string][]*attribute{}}
						vars[atomLoc] = r.bodyLocVar
					} else {
						r.bodyLocVar = r.headLocVar
					}
					addToHeadVarMapping(r.bodyLocVar)
				} else if r.bodyLocVar.id != atomLoc {
					return nil, newSemanticError("the location in all body atoms (where applicable) must be the same", astRule.Pos)
				}

				atomTime := astAtom.Variables[len(astAtom.Variables)-1].Name
				if r.bodyTimeVar == nil {
					if atomTime != r.headTimeVar.id {
						r.bodyTimeVar = &variable{id: atomTime, attrs: map[string][]*attribute{}}
						vars[atomTime] = r.bodyTimeVar
					} else {
						r.bodyTimeVar = r.headTimeVar
					}
					addToHeadVarMapping(r.bodyTimeVar)
				} else if r.bodyTimeVar.id != atomTime {
					return nil, newSemanticError("the time in all body atoms (where applicable) must be the same", astRule.Pos)
				}

				for j, astVar := range astAtom.Variables {
					if j >= len(astAtom.Variables)-2 {
						break
					}

					a := &attribute{
						index:    j,
						relation: rel,
					}

					var v *variable
					var ok bool
					if v, ok = vars[astVar.Name]; ok {
						v.attrs[rel.id] = append(v.attrs[rel.id], a)
					} else {
						v = &variable{
							id:    astVar.Name,
							attrs: map[string][]*attribute{rel.id: {a}},
						}
						vars[v.id] = v
					}
					addToHeadVarMapping(v)

					r.vars[astAtom.Name][j] = v
				}
			}

			for _, astCond := range conditions {
				v1, ok := vars[astCond.Var1.Name]
				if !ok {
					return nil, newSemanticError(fmt.Sprintf("all variables in conditions must show up in a positive atom: %q does not", astCond.Var1.Name), astCond.Var1.Pos)
				}
				v2, ok := vars[astCond.Var2.Name]
				if !ok {
					return nil, newSemanticError(fmt.Sprintf("all variables in conditions must show up in a positive atom: %q does not", astCond.Var2.Name), astCond.Var2.Pos)
				}
				r.conditions = append(r.conditions, condition{v1: v1, v2: v2, op: astCond.Operand})
			}

			for _, astAtom := range lateAtoms {
				switch astAtom.Name {
				case successorRelationName:
					if len(astAtom.Variables) != 2 || astAtom.Variables[0].Name != r.bodyTimeVar.id || astAtom.Variables[1].Name != r.headTimeVar.id {
						return nil, newSemanticError("incorrectly formatted successor relation", astAtom.Pos)
					}
					r.timeModel = timeModelSuccessor

				case chooseRelationName:
					if len(astAtom.Variables) != 2 {
						return nil, newSemanticError("choose relations must have exactly two attributes", astAtom.Pos)
					} else if astAtom.Variables[1].Name != r.headTimeVar.id {
						return nil, newSemanticError("the second variable in choose relations must be the head relation's time variable", astAtom.Variables[1].Pos)
					}

					t := astAtom.Variables[0].NameTuple
					fmt.Println(len(r.headVarMapping))
					if len(t) != len(r.headVarMapping) {
						return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
					}

					for i, v := range r.headVarMapping {
						if v.v.id != t[i].Name {
							return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", t[i].Pos)
						}
					}
					r.timeModel = timeModelAsync
				}
			}
		} else if astStatement.Preload != nil {
			astPreload := astStatement.Preload
			row := make([]string, len(astPreload.Fields))
			for i, f := range astPreload.Fields {
				// Per the lexer invariants, len(f.Data) >= 2.
				row[i] = f.Data[1 : len(f.Data)-1]
			}

			if _, ok := runner.relations[astPreload.Name]; !ok {
				return nil, newSemanticError("unreferenced relation found in a preload", astPreload.Pos)
			} else if len(row) != len(runner.relations[astPreload.Name].indexes) {
				return nil, newSemanticError("preload has a different number of attributes than the relation", astPreload.Pos)
			}

			runner.relations[astPreload.Name].push(row, astPreload.Loc, astPreload.Time)
			runner.locations[astPreload.Loc] = struct{}{}
		}
	}

	return &runner, nil
}

func (r *Runner) Step() {
	var queue []*rule
	inQueue := map[string]struct{}{}
	queue = append(queue, r.rules...)

	for _, rl := range queue {
		inQueue[rl.id] = struct{}{}
	}

	for len(queue) != 0 {
		rl := queue[0]
		delete(inQueue, rl.id)
		queue = queue[1:]

		time := r.currentTimestamp

		var data [][]string
		for loc := range r.locations {
			ldata := join(rl, loc, time)
			if rl.hasAggregation {
				ldata = aggregate(rl, ldata)
			}

			data = append(data, ldata...)
		}

		modified := false
		for _, d := range data {
			var nextTime int
			switch rl.timeModel {
			case timeModelSame:
				nextTime = time
			case timeModelSuccessor:
				nextTime = time + 1
			case timeModelAsync:
				combined := strings.Join(d, ";")
				b := big.NewInt(0)
				h := sha1.New()
				h.Write([]byte(combined))

				b.SetBytes(h.Sum(nil)[:7]) // h.Sum(nil) has a fixed size (sha1.Size).

				randSrc := rand.NewSource(b.Int64())
				nextTime = time + rand.New(randSrc).Intn(8)
			}

			tuple := d[:len(d)-1]
			nextLoc := d[len(d)-1]
			// Keep track of new locations
			r.locations[nextLoc] = struct{}{}

			fmt.Println(rl.head.id+":", tuple, nextLoc, nextTime)
			if rl.head.push(tuple, nextLoc, nextTime) {
				modified = true
			}
		}

		if modified {
			for _, bodyRule := range rl.head.bodyRules {
				if _, ok := inQueue[bodyRule.id]; !ok {
					queue = append(queue)
				}
			}
		}
	}

	r.currentTimestamp++
}

func (r *Runner) PrintRelation(name string) error {
	rel, ok := r.relations[name]
	if !ok {
		return nil
	}

	tabw := new(tabwriter.Writer)
	tabw.Init(os.Stdout, 4, 8, 1, ' ', 0)

	fmt.Fprint(tabw, "Idx\t")
	for i := range rel.indexes {
		fmt.Fprintf(tabw, "A%d\t", i)
	}
	fmt.Fprintln(tabw, "Loc\tTime")

	facts := rel.allAcrossSpaceTime()
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].timestamp == facts[j].timestamp {
			if facts[i].location <= facts[j].location {
				return true
			}
			return false
		} else if facts[i].timestamp < facts[j].timestamp {
			return true
		}
		return false
	})

	for i, f := range facts {
		fmt.Fprintf(tabw, "%d.\t", i+1)
		for _, val := range f.data {
			fmt.Fprintf(tabw, "%s\t", val)
		}
		fmt.Fprintf(tabw, "%s\t%d\n", f.location, f.timestamp)
	}
	return tabw.Flush()
}
