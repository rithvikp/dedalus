package fn

import "testing"

func TestFuncEval(t *testing.T) {
	tests := []struct {
		msg     string
		f       Func
		inputs  [][]int
		outputs []int
	}{
		{
			msg:     "identity function",
			f:       Identity(),
			inputs:  [][]int{{1}, {0}},
			outputs: []int{1, 0},
		},
		{
			msg:     "add function",
			f:       FromExpr(AddExp(IdentityExp(1), IdentityExp(3)), 5),
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

func TestFuncMergeDomain(t *testing.T) {
	tests := []struct {
		msg          string
		f            Func
		mergeIndices [][]int
		inputs       [][]int
		outputs      []int
	}{
		{
			msg:          "function that adds input elements",
			f:            FromExpr(AddExp(IdentityExp(0), IdentityExp(1)), 2),
			mergeIndices: [][]int{{0, 1}},
			inputs:       [][]int{{1}, {2}},
			outputs:      []int{2, 4},
		},
		{
			msg:          "function that adds input elements with multiple merges",
			f:            FromExpr(AddExp(AddExp(IdentityExp(0), IdentityExp(1)), IdentityExp(3)), 5),
			mergeIndices: [][]int{{0, 1}, {1, 3}},
			inputs:       [][]int{{1, 2, 3}, {2, 5, 6}},
			outputs:      []int{4, 9},
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
					t.Errorf("output diff: got %d, want %d for merged function %v", got, tt.outputs[i], tt.f)
				}
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		msg   string
		a     Func
		b     Func
		equal bool
	}{
		{
			msg:   "two identity functions",
			a:     Identity(),
			b:     Identity(),
			equal: true,
		},
		{
			msg:   "two functions that return different elements of the input",
			a:     FromExpr(IdentityExp(1), 2),
			b:     FromExpr(IdentityExp(0), 0),
			equal: false,
		},
		{
			msg:   "two equivalent add functions (but with different exp structures)",
			a:     FromExpr(AddExp(AddExp(Number(1), Number(2)), IdentityExp(1)), 2),
			b:     FromExpr(AddExp(IdentityExp(1), Number(3)), 2),
			equal: true,
		},
		{
			msg:   "two different add functions",
			a:     FromExpr(AddExp(AddExp(Number(3), Number(2)), IdentityExp(1)), 2),
			b:     FromExpr(AddExp(IdentityExp(1), Number(3)), 2),
			equal: false,
		},
		{
			msg:   "different domain sizes",
			a:     FromExpr(IdentityExp(0), 2),
			b:     FromExpr(IdentityExp(2), 3),
			equal: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			if Equal(tt.a, tt.b) != tt.equal {
				t.Errorf("expected functions to be equal: got %v\n\n want %v", tt.a, tt.b)
			}
		})
	}
}
