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

// TODO: validate for safe negation, auto-persisted relations

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

type attribute struct {
	relation *relation
	index    int
}

type variable struct {
	id    string
	attrs map[string][]*attribute
}

type assignment struct {
	v *variable
	e expression
}

type condition struct {
	e1 expression
	e2 expression
	op string
}

type expression interface {
	Eval(value func(v *variable) string) string
}

type binOp struct {
	e1 expression
	e2 expression
	op string
}

type number int

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
	assignments []assignment

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

func (c condition) Eval(value func(v *variable) string) bool {
	val1 := c.e1.Eval(value)
	val2 := c.e2.Eval(value)

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

func (v *variable) Eval(value func(v *variable) string) string {
	return value(v)
}

func (n number) Eval(value func(v *variable) string) string {
	return strconv.Itoa(int(n))
}

func (bo *binOp) Eval(value func(v *variable) string) string {
	e1 := bo.e1.Eval(value)
	e2 := bo.e2.Eval(value)

	v1i, v1f, float1, err := stringToNumber(e1)
	if err != nil {
		panic(err)
	}
	v2i, v2f, float2, err := stringToNumber(e2)
	if err != nil {
		panic(err)
	}
	float := float1 || float2

	switch bo.op {
	case "+":
		if !float {
			return strconv.Itoa(v1i + v2i)
		}
		return fmt.Sprintf("%f", v1f+v2f)
	case "-":
		if !float {
			return strconv.Itoa(v1i - v2i)
		}
		return fmt.Sprintf("%f", v1f-v2f)
	case "*":
		if !float {
			return strconv.Itoa(v1i - v2i)
		}
		return fmt.Sprintf("%f", v1f-v2f)
	}

	return ""
}

func NewRunner(p *ast.Program) (*Runner, error) {
	runner := Runner{
		relations: map[string]*relation{},
		locations: map[string]struct{}{},
	}

	// The rule argument is allowed to be nil if the relation addition is happening for a preload
	addRel := func(id string, vars int, pos lexer.Position, head, preload, readOnly bool, rl *rule) (*relation, error) {
		var ok bool
		var r *relation
		if r, ok = runner.relations[id]; !ok {
			lenOff := 0
			if !readOnly {
				lenOff = -2
			}

			if vars+lenOff < 0 {
				return nil, newSemanticError(fmt.Sprintf("%q, which is not a replicated read-only relation, must have time and location attributes", id), pos)
			}

			r = newRelation(id, readOnly, strings.ToUpper(id[0:1]) == id[0:1], vars+lenOff)

			// FIXME
			//r.autoPersist = !r.autoPersist

			runner.relations[id] = r
			if !head && rl != nil {
				r.bodyRules = append(r.bodyRules, rl)
			}
		} else {
			if r.readOnly && head {
				return nil, newSemanticError(fmt.Sprintf("%q, a read-only, relation cannot appear in the head of any rule", id), pos)
			}
			if !r.readOnly && r.numAttrs() != vars-2 || r.readOnly && r.numAttrs() != vars {
				return nil, newSemanticError(fmt.Sprintf("the number of attributes must be constant for any given relation, but %q had %d attributes initially and has %d attributes now", r.id, r.numAttrs(), vars), pos)
			}
			if !head && rl != nil {
				r.bodyRules = append(r.bodyRules, rl)
			}
		}

		return r, nil
	}

	var astRules []*ast.Rule
	for _, astStatement := range p.Statements {
		if astStatement.Rule != nil {
			astRules = append(astRules, astStatement.Rule)
		} else if astStatement.Preload != nil {
			astPreload := astStatement.Preload
			row := make([]string, len(astPreload.Fields))
			for i, f := range astPreload.Fields {
				// Per the lexer invariants, len(f.Data) >= 2.
				row[i] = f.Data[1 : len(f.Data)-1]
			}

			// This is necessary as addRel expects the `var` count to include time/loc if
			// applicable
			lenOff := 0
			if astPreload.Time != nil {
				lenOff = 2
			}

			rel, err := addRel(astPreload.Name, len(row)+lenOff, astPreload.Pos, false, true, astPreload.Time == nil, nil)
			if err != nil {
				return nil, err
			}

			if len(row) != rel.numAttrs() {
				return nil, newSemanticError("preload has a different number of attributes than the relation", astPreload.Pos)
			}

			if astPreload.Loc != nil && astPreload.Time != nil {
				rel.push(row, *astPreload.Loc, *astPreload.Time)
				runner.locations[*astPreload.Loc] = struct{}{}
			} else {
				rel.push(row, "", 0)
			}
		}
	}

	for i, astRule := range astRules {
		rl := &rule{
			id:             strconv.Itoa(i),
			headVarMapping: make([]headTerm, len(astRule.Head.Terms)-2),
		}
		vars := map[string]*variable{}
		rl.vars = map[string][]*variable{}

		astHeadVars := astRule.Head.Terms
		if len(astHeadVars) < 2 {
			return nil, newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astRule.Head.Name), astRule.Head.Pos)
		}

		var err error
		rl.head, err = addRel(astRule.Head.Name, len(astRule.Head.Terms), astRule.Pos, true, true, false, rl)
		if err != nil {
			return nil, err
		}
		rl.headLocVar = &variable{
			id:    astHeadVars[len(astHeadVars)-2].Variable.Name,
			attrs: map[string][]*attribute{},
		}
		vars[rl.headLocVar.id] = rl.headLocVar
		rl.headTimeVar = &variable{
			id:    astHeadVars[len(astHeadVars)-1].Variable.Name,
			attrs: map[string][]*attribute{},
		}
		vars[rl.headTimeVar.id] = rl.headTimeVar

		runner.rules = append(runner.rules, rl)

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

			rl.hasAggregation = true
			agg := aggregator(*v.Aggregator)
			if !agg.Valid() {
				return nil, newSemanticError(fmt.Sprintf("invalid aggregation function %q", agg), v.Pos)
			}
			aggregatedIndices[j] = agg
		}

		addToHeadVarMapping := func(v *variable) {
			if indices, ok := headVars[v.id]; ok {
				for _, k := range indices {
					hv := headTerm{v: v}
					if agg, ok := aggregatedIndices[k]; ok {
						hv.agg = &agg
					}
					rl.headVarMapping[k] = hv
				}
			}
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

			rel, err := addRel(astAtom.Name, len(astAtom.Variables), astAtom.Pos, false, true, false, rl)
			if err != nil {
				return nil, err
			}

			// TODO: Cleanup time/loc parsing (there are many "-2"'s when looking at # of variables
			// due to this issue). This is especially confusing due to intricacies with read-only
			// tables.
			if !rel.readOnly && len(astAtom.Variables) < 2 {
				return nil, newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astAtom.Name), astAtom.Pos)
			}

			if astAtom.Negated {
				rl.negatedBody = append(rl.negatedBody, rel)
			} else {
				rl.body = append(rl.body, rel)
			}
			rl.vars[astAtom.Name] = make([]*variable, 0, len(astAtom.Variables))

			if !rel.readOnly {
				atomLoc := astAtom.Variables[len(astAtom.Variables)-2].Name
				if rl.bodyLocVar == nil {
					if atomLoc != rl.headLocVar.id {
						rl.bodyLocVar = &variable{id: atomLoc, attrs: map[string][]*attribute{}}
						vars[atomLoc] = rl.bodyLocVar
					} else {
						rl.bodyLocVar = rl.headLocVar
					}
					addToHeadVarMapping(rl.bodyLocVar)
				} else if rl.bodyLocVar.id != atomLoc {
					return nil, newSemanticError("the location in all body atoms (where applicable) must be the same", astRule.Pos)
				}

				atomTime := astAtom.Variables[len(astAtom.Variables)-1].Name
				if rl.bodyTimeVar == nil {
					if atomTime != rl.headTimeVar.id {
						rl.bodyTimeVar = &variable{id: atomTime, attrs: map[string][]*attribute{}}
						vars[atomTime] = rl.bodyTimeVar
					} else {
						rl.bodyTimeVar = rl.headTimeVar
					}
					addToHeadVarMapping(rl.bodyTimeVar)
				} else if rl.bodyTimeVar.id != atomTime {
					return nil, newSemanticError("the time in all body atoms (where applicable) must be the same", astRule.Pos)
				}
			}

			for j, astVar := range astAtom.Variables {
				if j >= len(astAtom.Variables)-2 && !rel.readOnly {
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
					if v.id != "_" {
						vars[v.id] = v
					}
				}
				addToHeadVarMapping(v)

				rl.vars[astAtom.Name] = append(rl.vars[astAtom.Name], v)
			}
		}

		for _, astCond := range conditions {
			canBeAssignment := true
			isAssignment := false

			parseFirstTerm := func(astE *ast.Expression) (expression, error) {
				if astE.Var != nil {
					v, ok := vars[astE.Var.Name]
					if !ok && (!canBeAssignment || astE.Expr != nil) {
						return nil, newSemanticError(fmt.Sprintf("all variables in conditions must first show up in a positive atom or an assignment: %q does not", astE.Var.Name), astE.Var.Pos)
					} else if !ok && canBeAssignment && astE.Expr == nil {
						v = &variable{id: astE.Var.Name}
						addToHeadVarMapping(v)
						vars[v.id] = v
						isAssignment = true
						canBeAssignment = false
					}
					return v, nil
				} else {
					return number(*astE.Num), nil
				}
			}

			parseExpr := func(astE *ast.Expression) (expression, error) {
				var e expression
				v, err := parseFirstTerm(astE)
				if err != nil {
					return nil, err
				}
				e = v

				if astE.Expr != nil {
					v, err = parseFirstTerm(astE)
					if err != nil {
						return nil, err
					}

					var bo *binOp
					for astE.Expr != nil {
						if bo == nil {
							bo = &binOp{}
							e = bo
						} else {
							bo2 := &binOp{}
							bo.e2 = bo2
							bo = bo2
						}
						bo.e1 = v
						bo.op = *astE.Op
						astE = astE.Expr
					}

					v, err = parseFirstTerm(astE)
					if err != nil {
						return nil, err
					}
					bo.e2 = v
				}
				return e, nil
			}

			e1, err := parseExpr(&astCond.Expr1)
			if err != nil {
				return nil, err
			}
			e2, err := parseExpr(&astCond.Expr2)
			if err != nil {
				return nil, err
			}

			if isAssignment {
				if astCond.Operand != "=" {
					return nil, newSemanticError("assignments must use the \"=\" operator", astCond.Pos)
				}
				rl.assignments = append(rl.assignments, assignment{v: e1.(*variable), e: e2})
			} else {
				rl.conditions = append(rl.conditions, condition{e1: e1, e2: e2, op: astCond.Operand})
			}
		}

		for _, astAtom := range lateAtoms {
			switch astAtom.Name {
			case successorRelationName:
				if len(astAtom.Variables) != 2 || astAtom.Variables[0].Name != rl.bodyTimeVar.id || astAtom.Variables[1].Name != rl.headTimeVar.id {
					return nil, newSemanticError("incorrectly formatted successor relation", astAtom.Pos)
				}
				rl.timeModel = timeModelSuccessor

			case chooseRelationName:
				if len(astAtom.Variables) != 2 {
					return nil, newSemanticError("choose relations must have exactly two attributes", astAtom.Pos)
				} else if astAtom.Variables[1].Name != rl.headTimeVar.id {
					return nil, newSemanticError("the second variable in choose relations must be the head relation's time variable", astAtom.Variables[1].Pos)
				}

				if astAtom.Variables[0].Name != "_" {
					t := astAtom.Variables[0].NameTuple
					if len(t) != len(rl.headVarMapping) {
						return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
					}

					for i, v := range rl.headVarMapping {
						if v.v.id != t[i].Name {
							return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", t[i].Pos)
						}
					}
				}
				rl.timeModel = timeModelAsync
			}
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

	// TODO: Optimize automatic persistence
	for _, rel := range r.relations {
		if !rel.autoPersist {
			continue
		}

		for loc := range r.locations {
			for _, f := range rel.all(loc, r.currentTimestamp) {
				rel.push(f.data, f.location, f.timestamp+1)
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
