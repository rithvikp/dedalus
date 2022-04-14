package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/rithvikp/dedalus/ast"
	"golang.org/x/exp/slices"
)

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

	locations map[string]struct{}
	executed  bool
}

func (s *State) Rules() []*Rule {
	return s.rules
}

func (s *State) NonEDBRelations() []*Relation {
	// TODO: Move unused relation removal to the compilation phase
	var rels []*Relation
	for _, r := range s.relations {
		if len(r.Rules()) > 0 && !r.IsEDB() {
			rels = append(rels, r)
		}
	}
	return rels
}

func (r *Rule) ID() string {
	return r.id
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

func (r *Rule) HeadVars() []*Variable {
	return r.vars[r.head.id]
}

func (r *Rule) VarOfAttr(a Attribute) (*Variable, bool) {
	if vars, ok := r.vars[a.relation.id]; ok {
		if vars[a.index].constant {
			return nil, false
		}
		return vars[a.index], true
	}

	return r.headVarMapping[a.index].v, true
}

func (r *Rule) ConstOfAttr(a Attribute) (int, bool) {
	if vars, ok := r.vars[a.relation.id]; ok {
		if !vars[a.index].constant {
			return 0, false
		}

		// TODO: Store a mapping instead of searching
		for _, c := range r.conditions {
			if expV, ok := c.e1.(*Variable); ok && expV == vars[a.index] {
				return int(c.e2.(number)), true
			}
		}
	}

	return 0, false
}

func (rl *Rule) String() string {
	b := strings.Builder{}

	// The head relation is a special-case, so handle it separately.
	rel := rl.head
	b.WriteString(fmt.Sprintf("%s(", rel.ID()))
	for j, ht := range rl.headVarMapping {
		if ht.agg != nil {
			b.WriteString(fmt.Sprintf("%s<%s>", *ht.agg, ht.v.id))
		} else {
			b.WriteString(ht.v.id)
		}
		if j < len(rl.headVarMapping)-1 {
			b.WriteString(",")
		}
	}
	if !rel.readOnly {
		locVar := rl.headLocVar
		timeVar := rl.headTimeVar
		b.WriteString(fmt.Sprintf(",%s,%s", locVar, timeVar))
	}
	b.WriteString(") :- ")

	for i, rel := range rl.body {
		b.WriteString(fmt.Sprintf("%s(", rel.ID()))
		for j, v := range rl.vars[rel.ID()] {
			// TODO: Support printing out constants
			b.WriteString(v.id)
			if j < len(rl.vars[rel.ID()])-1 {
				b.WriteString(",")
			}
		}

		if !rel.readOnly {
			locVar := rl.bodyLocVar
			timeVar := rl.bodyTimeVar
			b.WriteString(fmt.Sprintf(",%s,%s", locVar, timeVar))
		}

		b.WriteString(")")
		if i < len(rl.body)-1 {
			b.WriteString(", ")
		}
	}

	switch rl.timeModel {
	case TimeModelSuccessor:
		b.WriteString(fmt.Sprintf(", %s(%s,%s)", successorRelationName, rl.bodyTimeVar, rl.headTimeVar))
	case TimeModelAsync:
		b.WriteString(fmt.Sprintf(", %s((", chooseRelationName))
		for i, ht := range rl.headVarMapping {
			// TODO: Handle aggregations
			if ht.agg != nil {
				b.WriteString(fmt.Sprintf("%s<%s>", *ht.agg, ht.v.id))
			} else {
				b.WriteString(ht.v.id)
			}
			if i < len(rl.headVarMapping)-1 {
				b.WriteString(",")
			}
		}
		b.WriteString(fmt.Sprintf("),%s)", rl.headTimeVar))
	}

	return b.String()
}

func New(p *ast.Program) (*State, error) {
	state := State{
		relations: map[string]*Relation{},
		locations: map[string]struct{}{},
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

			rel, err := state.addRel(astPreload.Name, len(row)+lenOff, astPreload.Pos, false, astPreload.Time == nil, nil)
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
		state.addRule(astRule, strconv.Itoa(i))
	}

	return &state, nil
}

// The rule argument is allowed to be nil if the relation addition is happening for a preload
func (s *State) addRel(id string, numVars int, pos lexer.Position, head, readOnly bool, rl *Rule) (*Relation, error) {
	var ok bool
	var rel *Relation
	if rel, ok = s.relations[id]; !ok {
		lenOff := 0
		if !readOnly {
			lenOff = -2
		}

		if numVars+lenOff < 0 {
			return nil, newSemanticError(fmt.Sprintf("%q, which is not a replicated read-only relation, must have time and location attributes", id), pos)
		}

		rel = newRelation(id, readOnly, strings.ToUpper(id[0:1]) == id[0:1], numVars+lenOff)

		s.relations[id] = rel
	} else {
		if rel.readOnly && head {
			return nil, newSemanticError(fmt.Sprintf("%q, a read-only relation cannot appear in the head of any rule", id), pos)
		}
		if !rel.readOnly && rel.numAttrs() != numVars-2 || rel.readOnly && rel.numAttrs() != numVars {
			return nil, newSemanticError(fmt.Sprintf("the number of attributes must be constant for any given relation, but %q had %d attributes initially and has %d attributes now", rel.id, rel.numAttrs(), numVars), pos)
		}
	}

	if rl != nil {
		if head {
			rel.headRules = append(rel.headRules, rl)
		} else {
			rel.bodyRules = append(rel.bodyRules, rl)
		}
	}

	return rel, nil
}

// This is a temporary shim until a rule builder is added.
func (s *State) AddRawRule(rawRule string) error {
	if s.executed {
		return errors.New("cannot add a rule to an engine state that has already been executed")
	}
	p, err := ast.Parse(strings.NewReader(rawRule))
	if err != nil {
		return err
	}

	if len(p.Statements) != 1 || p.Statements[0].Rule == nil {
		return errors.New("the provided raw string was not actually a single rule")
	}

	return s.addRule(p.Statements[0].Rule, strconv.Itoa(len(s.rules)))
}

func (s *State) addRule(astRule *ast.Rule, id string) error {
	rl := &Rule{
		id:             id,
		headVarMapping: make([]headTerm, len(astRule.Head.Terms)-2),
	}
	vars := map[string]*Variable{}
	rl.vars = map[string][]*Variable{}

	astHeadVars := astRule.Head.Terms
	if len(astHeadVars) < 2 {
		return newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astRule.Head.Name), astRule.Head.Pos)
	}

	var err error
	rl.head, err = s.addRel(astRule.Head.Name, len(astRule.Head.Terms), astRule.Pos, true, false, rl)
	if err != nil {
		return err
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

	s.rules = append(s.rules, rl)

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
				return newSemanticError(fmt.Sprintf("invalid aggregation function %q", agg), astVar.Pos)
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

		rel, err := s.addRel(astAtom.Name, len(astAtom.Terms), astAtom.Pos, false, false, rl)
		if err != nil {
			return err
		}

		// TODO: Cleanup time/loc parsing (there are many "-2"'s when looking at # of variables
		// due to this issue). This is especially confusing due to intricacies with read-only
		// tables.
		if !rel.readOnly && len(astAtom.Terms) < 2 {
			return newSemanticError(fmt.Sprintf("%q is not a replicated read-only relation so must have time and location attributes", astAtom.Name), astAtom.Pos)
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
				return newSemanticError("the location in all body atoms (where applicable) must be the same", astRule.Pos)
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
				return newSemanticError("the time in all body atoms (where applicable) must be the same", astRule.Pos)
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
			return err
		}
		e2, err := parseExpr(&astCond.Expr2)
		if err != nil {
			return err
		}

		if isAssignment {
			if astCond.Operand != "=" {
				return newSemanticError("assignments must use the \"=\" operator", astCond.Pos)
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
				return newSemanticError("all terms in a time relation must be variables", t.Pos)
			}
		}

		switch astAtom.Name {
		case successorRelationName:
			if len(terms) != 2 || terms[0].Var.Name != rl.bodyTimeVar.id || terms[1].Var.Name != rl.headTimeVar.id {
				return newSemanticError("incorrectly formatted successor relation", astAtom.Pos)
			}
			rl.timeModel = TimeModelSuccessor

		case chooseRelationName:
			if len(terms) != 2 {
				return newSemanticError("choose relations must have exactly two attributes", astAtom.Pos)
			} else if terms[1].Var.Name != rl.headTimeVar.id {
				return newSemanticError("the second variable in choose relations must be the head relation's time variable", terms[1].Var.Pos)
			}

			if terms[0].Var.Name != "_" {
				t := terms[0].Var.NameTuple
				if len(t) != len(rl.headVarMapping) {
					return newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
				}

				// TODO: Support aggregations
				for i, v := range rl.headVarMapping {
					if v.v.id != t[i].Name {
						return newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", t[i].Pos)
					}
				}
			} else {
				return newSemanticError("the first element of a choose relation must be a tuple of all the corresponding head variables (in the same order as in the head)", astAtom.Pos)
			}
			rl.timeModel = TimeModelAsync
		}
	}

	for i, ht := range rl.headVarMapping {
		if ht.v == nil {
			return newSemanticError(fmt.Sprintf("variable %d of the head does not appear in the body", i), astRule.Head.Pos)
		}
	}
	return nil
}
