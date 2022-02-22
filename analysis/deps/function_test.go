package deps

import "testing"

func TestFunctionEval(t *testing.T) {
	tests := []struct {
		msg     string
		f       Function
		inputs  [][]int
		outputs []int
	}{
		{
			msg:     "identity function",
			f:       IdentityFunc(),
			inputs:  [][]int{{1}, {0}},
			outputs: []int{1, 0},
		},
		{
			msg:     "add function",
			f:       ExprFunc(AddExp(IdentityExp(1), IdentityExp(3)), 5),
			inputs:  [][]int{{1, 2, 3, 4, 5}, {0, -1, -5, -6, 10}},
			outputs: []int{6, -7},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			for i, input := range tt.inputs {
				got := tt.f.Eval(input)

				if got != tt.outputs[i] {
					t.Errorf("output diff: got %d, want %d", got, tt.outputs[i])
				}
			}
		})
	}
}

func TestFunctionMergeDomain(t *testing.T) {
	tests := []struct {
		msg          string
		f            Function
		mergeIndices [][]int
		inputs       [][]int
		outputs      []int
	}{
		{
			msg:          "function that adds input elements",
			f:            ExprFunc(AddExp(IdentityExp(0), IdentityExp(1)), 2),
			mergeIndices: [][]int{{0, 1}},
			inputs:       [][]int{{1}, {2}},
			outputs:      []int{2, 4},
		},
		{
			msg:          "function that adds input elements with multiple merges",
			f:            ExprFunc(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(3)), 5),
			mergeIndices: [][]int{{0, 1}, {1, 3}},
			inputs:       [][]int{{1, 2, 3}, {2, 5, 6}},
			outputs:      []int{5, 10},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			for _, merge := range tt.mergeIndices {
				tt.f.MergeDomain(merge)
			}
			for i, input := range tt.inputs {
				got := tt.f.Eval(input)

				if got != tt.outputs[i] {
					t.Errorf("output diff: got %d, want %d", got, tt.outputs[i])
				}
			}
		})
	}
}

func TestFuncEqual(t *testing.T) {
	tests := []struct {
		msg   string
		a     Function
		b     Function
		equal bool
	}{
		{
			msg:   "two identity functions",
			a:     IdentityFunc(),
			b:     IdentityFunc(),
			equal: true,
		},
		{
			msg:   "two functions that return different elements of the input",
			a:     ExprFunc(IdentityExp(1), 2),
			b:     ExprFunc(IdentityExp(0), 0),
			equal: false,
		},
		{
			msg:   "two equivalent add functions (but with different exp structures)",
			a:     ExprFunc(AddExp(AddExp(number(1), number(2)), IdentityExp(1)), 2),
			b:     ExprFunc(AddExp(IdentityExp(1), number(3)), 2),
			equal: true,
		},
		{
			msg:   "two different add functions",
			a:     ExprFunc(AddExp(AddExp(number(3), number(2)), IdentityExp(1)), 2),
			b:     ExprFunc(AddExp(IdentityExp(1), number(3)), 2),
			equal: false,
		},
		{
			msg:   "different domain sizes",
			a:     ExprFunc(IdentityExp(0), 2),
			b:     ExprFunc(IdentityExp(2), 3),
			equal: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			if funcEqual(tt.a, tt.b) != tt.equal {
				// TODO: Add a string representation of the functions to the error message
				t.Errorf("expected functions to be equal")
			}
		})
	}
}
