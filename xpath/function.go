package xpath

import (
	"fmt"
	"strings"
)

var xpathfunctions map[string]*Function

const (
	fnNS = "http://www.w3.org/2005/xpath-functions"
)

func fnBoolean(args []Sequence) (Sequence, error) {
	bv, err := booleanValue(args[0])
	return Sequence{bv}, err
}

func fnConcat(args []Sequence) (Sequence, error) {
	var str []string

	for _, seq := range args {
		str = append(str, seq.stringvalue())
	}
	return Sequence{strings.Join(str, "")}, nil
}

func fnCount(args []Sequence) (Sequence, error) {
	seq := args[0]
	return Sequence{len(seq)}, nil
}

func fnFalse(args []Sequence) (Sequence, error) {
	return Sequence{false}, nil
}

func fnNot(args []Sequence) (Sequence, error) {
	b, err := booleanValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{!b}, nil
}

func fnNumber(args []Sequence) (Sequence, error) {
	bv, err := numberValue(args[0])
	return Sequence{bv}, err
}

func fnTrue(args []Sequence) (Sequence, error) {
	return Sequence{true}, nil
}

func init() {
	xpathfunctions = make(map[string]*Function)

	RegisterFunction(&Function{Name: "boolean", Namespace: fnNS, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: fnNS, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "concat", Namespace: fnNS, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "count", Namespace: fnNS, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})
	RegisterFunction(&Function{Name: "not", Namespace: fnNS, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "true", Namespace: fnNS, F: fnTrue})

}

// Function represents an XPath function
type Function struct {
	Name      string
	Namespace string
	F         func([]Sequence) (Sequence, error)
	MinArg    int
	MaxArg    int
}

// RegisterFunction registers an XPath function
func RegisterFunction(f *Function) {
	xpathfunctions[f.Name] = f
}

func getfunction(name string) *Function {
	return xpathfunctions[name]
}

func hasFunction(name string) bool {
	_, ok := xpathfunctions[name]
	return ok
}

func callFunction(name string, arguments []Sequence) (Sequence, error) {
	fn := getfunction(name)

	if min := fn.MinArg; min > 0 {
		if len(arguments) < min {
			return nil, fmt.Errorf("too few arguments in function call (%q), min: %d", fn.Name, fn.MinArg)
		}
	}
	if max := fn.MaxArg; max > -1 {
		if len(arguments) > max {
			return nil, fmt.Errorf("too many arguments in function call (%q), max: %d, got %d (%#v)", fn.Name, fn.MaxArg, len(arguments), arguments)
		}
	}
	return fn.F(arguments)
}
