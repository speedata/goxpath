package xpath

import (
	"fmt"
	"strings"
)

var xpathfunctions map[string]*Function

const (
	fnNS = "http://www.w3.org/2005/xpath-functions"
)

func fnBoolean(args []sequence) (sequence, error) {
	bv, err := booleanValue(args[0])
	return sequence{bv}, err
}

func fnConcat(args []sequence) (sequence, error) {
	var str []string
	for _, seq := range args {
		str = append(str, seq.stringvalue())
	}
	return sequence{strings.Join(str, "")}, nil
}

func fnFalse(args []sequence) (sequence, error) {
	return sequence{false}, nil
}

func fnNot(args []sequence) (sequence, error) {
	b, err := booleanValue(args[0])
	if err != nil {
		return nil, err
	}
	return sequence{!b}, nil
}

func fnNumber(args []sequence) (sequence, error) {
	bv, err := numberValue(args[0])
	return sequence{bv}, err
}

func fnTrue(args []sequence) (sequence, error) {
	return sequence{true}, nil
}

func init() {
	xpathfunctions = make(map[string]*Function)

	RegisterFunction(&Function{Name: "boolean", Namespace: fnNS, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: fnNS, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "concat", Namespace: fnNS, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})
	RegisterFunction(&Function{Name: "not", Namespace: fnNS, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "true", Namespace: fnNS, F: fnTrue})

}

// Function represents an XPath function
type Function struct {
	Name      string
	Namespace string
	F         func([]sequence) (sequence, error)
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

func callFunction(name string, arguments []sequence) (sequence, error) {
	fn := getfunction(name)

	if min := fn.MinArg; min > 0 {
		if len(arguments) < min {
			return nil, fmt.Errorf("too few arguments in function call (%q), min: %d", fn.Name, fn.MinArg)
		}
	}
	if max := fn.MaxArg; max > -1 {
		if len(arguments) > max {
			return nil, fmt.Errorf("too many arguments in function call (%q), max: %d", fn.Name, fn.MaxArg)
		}
	}

	return fn.F(arguments)
}
