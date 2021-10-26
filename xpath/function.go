package xpath

import (
	"fmt"
	"math"
	"strings"

	"github.com/speedata/goxml"
)

var xpathfunctions map[string]*Function

const (
	fnNS = "http://www.w3.org/2005/xpath-functions"
)

func fnAbs(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := numberValue(seq)
	return Sequence{math.Abs(itm)}, err
}

func fnBoolean(ctx *Context, args []Sequence) (Sequence, error) {
	bv, err := booleanValue(args[0])
	return Sequence{bv}, err
}

func fnCeiling(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := numberValue(seq)
	return Sequence{math.Ceil(itm)}, err
}

func fnConcat(ctx *Context, args []Sequence) (Sequence, error) {
	var str []string

	for _, seq := range args {
		str = append(str, seq.stringvalue())
	}
	return Sequence{strings.Join(str, "")}, nil
}

func fnCount(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	return Sequence{len(seq)}, nil
}

func fnEmpty(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{len(args[0]) == 0}, nil
}

func fnFalse(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{false}, nil
}

func fnFloor(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := numberValue(seq)
	return Sequence{math.Floor(itm)}, err
}

func fnLast(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.size}, nil
}

func fnLocalName(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.context
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if elt, ok := arg[0].(*goxml.Element); ok {
		return Sequence{elt.Name}, nil
	}
	return Sequence{""}, nil
}

func fnNot(ctx *Context, args []Sequence) (Sequence, error) {
	b, err := booleanValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{!b}, nil
}

func fnNumber(ctx *Context, args []Sequence) (Sequence, error) {
	bv, err := numberValue(args[0])
	return Sequence{bv}, err
}

func fnPosition(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.pos}, nil
}

func fnString(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.context
	} else {
		arg = args[0]
	}
	return Sequence{arg.stringvalue()}, nil
}

func fnTrue(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{true}, nil
}

func init() {
	xpathfunctions = make(map[string]*Function)

	RegisterFunction(&Function{Name: "abs", Namespace: fnNS, F: fnAbs, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: fnNS, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ceiling", Namespace: fnNS, F: fnCeiling, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "concat", Namespace: fnNS, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "count", Namespace: fnNS, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "empty", Namespace: fnNS, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: fnNS, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "last", Namespace: fnNS, F: fnLast})
	RegisterFunction(&Function{Name: "local-name", Namespace: fnNS, F: fnLocalName, MinArg: 0, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: fnNS, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: fnNS, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: fnNS, F: fnPosition})
	RegisterFunction(&Function{Name: "string", Namespace: fnNS, F: fnString, MinArg: 0, MaxArg: 1})
	RegisterFunction(&Function{Name: "true", Namespace: fnNS, F: fnTrue})
}

// Function represents an XPath function
type Function struct {
	Name      string
	Namespace string
	F         func(*Context, []Sequence) (Sequence, error)
	MinArg    int
	MaxArg    int
}

// RegisterFunction registers an XPath function
func RegisterFunction(f *Function) {
	xpathfunctions[f.Name] = f
}

func getfunction(name string) *Function {
	// todo: namespace handling etc.
	return xpathfunctions[name]
}

func hasFunction(name string) bool {
	_, ok := xpathfunctions[name]
	return ok
}

func callFunction(name string, arguments []Sequence, ctx *Context) (Sequence, error) {
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
	return fn.F(ctx, arguments)
}
