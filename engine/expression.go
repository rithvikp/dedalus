package engine

import (
	"fmt"
	"strconv"
)

type expression interface {
	eval(value func(v *Variable) string) string
}

type binOp struct {
	e1 expression
	e2 expression
	op string
}

type number int

type assignment struct {
	v *Variable
	e expression
}

type condition struct {
	e1 expression
	e2 expression
	op string
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

func (c condition) eval(value func(v *Variable) string) bool {
	val1 := c.e1.eval(value)
	val2 := c.e2.eval(value)

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

func (v *Variable) eval(value func(v *Variable) string) string {
	return value(v)
}

func (n number) eval(value func(v *Variable) string) string {
	return strconv.Itoa(int(n))
}

func (bo *binOp) eval(value func(v *Variable) string) string {
	e1 := bo.e1.eval(value)
	e2 := bo.e2.eval(value)

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
