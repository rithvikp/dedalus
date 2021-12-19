package ast

type table struct {
	data map[string]string
}

type state struct {
	relations map[string]table
}

func (r *Rule) eval(s *state) {

}
