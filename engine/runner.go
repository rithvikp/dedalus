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
	"golang.org/x/exp/slices"
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

type TimeModel int

const (
	TimeModelSame = iota
	TimeModelSuccessor
	TimeModelAsync
)

const (
	successorRelationName = "succ"
	chooseRelationName    = "choose"
)

var lateHandleAtoms = map[string]struct{}{
	successorRelationName: {},
	chooseRelationName:    {},
}

type headTerm struct {
	agg *aggregator // Optional
	v   *Variable
}

type Rule struct {
	id          string
	head        *Relation
	body        []*Relation
	negatedBody []*Relation

	conditions []condition
	// Directly assign head variables which don't appear in the body.
	assignments []assignment

	// The index in the head relation mapped to the corresponding variable in the body.
	headVarMapping []headTerm

	// The keys are relations and the list of variables corresponds to the variables in the atom.
	vars map[string][]*Variable

	timeModel   TimeModel
	bodyLocVar  *Variable
	headLocVar  *Variable
	bodyTimeVar *Variable
	headTimeVar *Variable

	hasAggregation bool
}

type State struct {
	relations map[string]*Relation

	rules []*Rule

	currentTimestamp int
	locations        map[string]struct{}
}

func (s *State) Rules() []*Rule {
	return s.rules
}

type Runner struct {
	*State
}

func (r *Rule) Body() []*Relation {
	body := make([]*Relation, len(r.body))
	copy(body, r.body)
	return body
}

func (r *Rule) IsNegated(rel *Relation) bool {
	return slices.Contains(r.negatedBody, rel)
}

func (r *Rule) Head() *Relation {
	return r.head
}

func (r *Rule) VarOfAttr(a Attribute) *Variable {
	if vars, ok := r.vars[a.relation.id]; ok {
		return vars[a.index]
	}

	return r.headVarMapping[a.index].v
}

func (v *Variable) Attrs() []Attribute {
	var attrs []Attribute
	for _, as := range v.attrs {
		attrs = append(attrs, as...)
	}
	return attrs
}

func New(p *ast.Program) (*State, error) {
	state := State{
		relations: map[string]*Relation{},
		locations: map[string]struct{}{},
	}

	// The rule argument is allowed to be nil if the relation addition is happening for a preload
	addRel := func(id string, vars int, pos lexer.Position, head, preload, readOnly bool, rl *Rule) (*Relation, error) {
		var ok bool
		var r *Relation
		if r, ok = state.relations[id]; !ok {
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

			state.relations[id] = r
			if rl != nil {
				if head {
					r.headRules = append(r.headRules, rl)
				} else {
					r.bodyRules = append(r.bodyRules, rl)
				}
			}
		} else {
			if r.readOnly && head {
				return nil, newSemanticError(fmt.Sprintf("%q, a read-only, relation cannot appear in the head of any rule", id), pos)
			}
			if !r.readOnly && r.numAttrs() != vars-2 || r.readOnly && r.numAttrs() != vars {
				return nil, newSemanticError(fmt.Sprintf("the number of attributes must be constant for any given relation, but %q had %d attributes initially and has %d attributes now", r.id, r.numAttrs(), vars), pos)
			}
			if rl != nil {
				if head {
					r.headRules = append(r.headRules, rl)
				} else {
					r.bodyRules = append(r.bodyRules, rl)
				}
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
				state.locations[*astPreload.Loc] = struct{}{}
			} else {
				rel.push(row, "", 0)
			}
		}
	}

	for i, astRule := range astRules {
		rl := &Rule{
			id:             strconv.Itoa(i),
			headVarMapping: make([]headTerm, len(astRule.Head.Terms)-2),
		}
		vars := map[string]*Variable{}
		rl.vars = map[string][]*Variable{}

		astHeadVars := astRule.Head.Terms
		if len(astHeadVars) < 2 {
			return nil, newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astRule.Head.Name), astRule.Head.Pos)
		}

		var err error
		rl.head, err = addRel(astRule.Head.Name, len(astRule.Head.Terms), astRule.Pos, true, true, false, rl)
		if err != nil {
			return nil, err
		}
		rl.headLocVar = &Variable{
			id:    astHeadVars[len(astHeadVars)-2].Variable.Name,
			attrs: map[string][]Attribute{},
		}
		vars[rl.headLocVar.id] = rl.headLocVar
		rl.headTimeVar = &Variable{
			id:    astHeadVars[len(astHeadVars)-1].Variable.Name,
			attrs: map[string][]Attribute{},
		}
		vars[rl.headTimeVar.id] = rl.headTimeVar

		state.rules = append(state.rules, rl)

		headVars := map[string][]int{}
		aggregatedIndices := map[int]aggregator{}
		addToHeadVarMapping := func(v *Variable) {
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

		addVariable := func(rel *Relation, vName string, a Attribute, addToHeadMapping bool, constant bool) {
			var v *Variable
			var ok bool
			if v, ok = vars[vName]; ok {
				v.attrs[rel.id] = append(v.attrs[rel.id], a)
			} else {
				v = &Variable{
					id:       vName,
					attrs:    map[string][]Attribute{rel.id: {a}},
					constant: constant,
				}
				if v.id != "_" {
					vars[v.id] = v
				}
			}

			if addToHeadMapping {
				addToHeadVarMapping(v)
			}
			rl.vars[rel.id] = append(rl.vars[rel.id], v)
		}

		for j, astVar := range astHeadVars {
			if j >= len(astHeadVars)-2 {
				break
			}

			headVars[astVar.Variable.Name] = append(headVars[astVar.Variable.Name], j)
			if astVar.Aggregator != nil {
				rl.hasAggregation = true
				agg := aggregator(*astVar.Aggregator)
				if !agg.Valid() {
					return nil, newSemanticError(fmt.Sprintf("invalid aggregation function %q", agg), astVar.Pos)
				}
				aggregatedIndices[j] = agg
			}

			rel := rl.head
			a := Attribute{
				index:    j,
				relation: rel,
			}
			addVariable(rel, astVar.Variable.Name, a, false, false)
		}

		var lateAtoms []*ast.Atom
		var conditions []*ast.Condition
		var constAssignments []struct {
			Name string
			Val  int
		} // Fake assignments used for constants in atoms
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

			rel, err := addRel(astAtom.Name, len(astAtom.Terms), astAtom.Pos, false, true, false, rl)
			if err != nil {
				return nil, err
			}

			// TODO: Cleanup time/loc parsing (there are many "-2"'s when looking at # of variables
			// due to this issue). This is especially confusing due to intricacies with read-only
			// tables.
			if !rel.readOnly && len(astAtom.Terms) < 2 {
				return nil, newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astAtom.Name), astAtom.Pos)
			}

			if astAtom.Negated {
				rl.negatedBody = append(rl.negatedBody, rel)
			} else {
				rl.body = append(rl.body, rel)
			}
			rl.vars[astAtom.Name] = make([]*Variable, 0, len(astAtom.Terms))

			termVars := make([]ast.Variable, len(astAtom.Terms))
			constTerms := map[string]bool{}
			for i, t := range astAtom.Terms {
				if t.Var != nil {
					termVars[i] = *t.Var
					continue
				}
				v := fmt.Sprintf("_rl-%s_%s_%d", rl.id, astAtom.Name, i)
				termVars[i] = ast.Variable{
					Pos:  t.Pos,
					Name: v,
				}
				constTerms[v] = true

				// TODO: Support more than just numeric constants
				if t.Num != nil {
					constAssignments = append(constAssignments, struct {
						Name string
						Val  int
					}{v, *t.Num})
				} else {
					panic("Internal Error: A term must always have either a variable or constant defined")
				}

			}

			if !rel.readOnly {
				atomLoc := termVars[len(termVars)-2].Name
				// TODO: Consolidate logic between this and addVariable()
				if rl.bodyLocVar == nil {
					if atomLoc != rl.headLocVar.id {
						rl.bodyLocVar = &Variable{
							id:       atomLoc,
							attrs:    map[string][]Attribute{},
							constant: constTerms[atomLoc],
						}
						vars[atomLoc] = rl.bodyLocVar
					} else {
						rl.bodyLocVar = rl.headLocVar
					}
					addToHeadVarMapping(rl.bodyLocVar)
				} else if rl.bodyLocVar.id != atomLoc {
					return nil, newSemanticError("the location in all body atoms (where applicable) must be the same", astRule.Pos)
				}

				atomTime := termVars[len(termVars)-1].Name
				if rl.bodyTimeVar == nil {
					if atomTime != rl.headTimeVar.id {
						rl.bodyTimeVar = &Variable{
							id:       atomTime,
							attrs:    map[string][]Attribute{},
							constant: constTerms[atomLoc],
						}
						vars[atomTime] = rl.bodyTimeVar
					} else {
						rl.bodyTimeVar = rl.headTimeVar
					}
					addToHeadVarMapping(rl.bodyTimeVar)
				} else if rl.bodyTimeVar.id != atomTime {
					return nil, newSemanticError("the time in all body atoms (where applicable) must be the same", astRule.Pos)
				}
			}

			for j, astVar := range termVars {
				if j >= len(termVars)-2 && !rel.readOnly {
					break
				}

				a := Attribute{
					index:    j,
					relation: rel,
				}
				addVariable(rel, astVar.Name, a, true, constTerms[astVar.Name])
			}
		}

		for _, astCond := range conditions {
			canBeAssignment := true
			isAssignment := false

			parseFirstTerm := func(astE *ast.Expression) (expression, error) {
				if astE.Var != nil {
					v, ok := vars[astE.Var.Name]
					if !ok {
						return nil, newSemanticError(fmt.Sprintf("all variables must appear in at least one atom: %q does not", astE.Var.Name), astE.Var.Pos)
					}
					_, inHead := v.attrs[rl.head.id]
					onlyInHead := len(v.attrs) == 1 && inHead
					if onlyInHead && (!canBeAssignment || astE.Expr != nil) {
						return nil, newSemanticError(fmt.Sprintf("all variables in conditions must first show up in a positive atom or an assignment: %q does not", astE.Var.Name), astE.Var.Pos)
					} else if onlyInHead && canBeAssignment && astE.Expr == nil {
						v = &Variable{id: astE.Var.Name}
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
				rl.assignments = append(rl.assignments, assignment{v: e1.(*Variable), e: e2})
			} else {
				rl.conditions = append(rl.conditions, condition{e1: e1, e2: e2, op: astCond.Operand})
			}
		}

		for _, a := range constAssignments {
			rl.conditions = append(rl.conditions, condition{e1: vars[a.Name], e2: number(a.Val), op: "="})
		}

		for _, astAtom := range lateAtoms {
			terms := astAtom.Terms
			for _, t := range terms {
				if t.Var == nil {
					return nil, newSemanticError("all terms in a time relation must be variables", t.Pos)
				}
			}

			switch astAtom.Name {
			case successorRelationName:
				if len(terms) != 2 || terms[0].Var.Name != rl.bodyTimeVar.id || terms[1].Var.Name != rl.headTimeVar.id {
					return nil, newSemanticError("incorrectly formatted successor relation", astAtom.Pos)
				}
				rl.timeModel = TimeModelSuccessor

			case chooseRelationName:
				if len(terms) != 2 {
					return nil, newSemanticError("choose relations must have exactly two attributes", astAtom.Pos)
				} else if terms[1].Var.Name != rl.headTimeVar.id {
					return nil, newSemanticError("the second variable in choose relations must be the head relation's time variable", terms[1].Var.Pos)
				}

				if terms[0].Var.Name != "_" {
					t := terms[0].Var.NameTuple
					if len(t) != len(rl.headVarMapping) {
						return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
					}

					for i, v := range rl.headVarMapping {
						if v.v.id != t[i].Name {
							return nil, newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", t[i].Pos)
						}
					}
				}
				rl.timeModel = TimeModelAsync
			}
		}

		for i, ht := range rl.headVarMapping {
			if ht.v == nil {
				return nil, newSemanticError(fmt.Sprintf("variable %d of the head does not appear in the body", i), astRule.Head.Pos)
			}
		}
	}

	return &state, nil
}

func NewRunner(p *ast.Program) (*Runner, error) {
	s, err := New(p)
	if err != nil {
		return nil, err
	}

	return &Runner{State: s}, nil
}

func (r *Runner) Step() {
	var queue []*Rule
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
			case TimeModelSame:
				nextTime = time
			case TimeModelSuccessor:
				nextTime = time + 1
			case TimeModelAsync:
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
