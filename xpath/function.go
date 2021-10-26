package xpath

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/speedata/goxml"
)

var (
	xpathfunctions   map[string]*Function
	multipleWSRegexp *regexp.Regexp
)

const (
	fnNS = "http://www.w3.org/2005/xpath-functions"
	xsNS = "http://www.w3.org/2001/XMLSchema"
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
	if attr, ok := arg[0].(*goxml.Attribute); ok {
		return Sequence{attr.Name}, nil
	}
	return Sequence{""}, nil
}

func fnMax(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := numberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := numberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		m = math.Max(m, ai)
	}
	return Sequence{m}, nil
}

func fnMin(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := numberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := numberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		m = math.Min(m, ai)
	}
	return Sequence{m}, nil
}

func fnNormalizeSpace(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.context
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if len(arg) > 1 {
		return nil, fmt.Errorf("The cardinality of first argument of fn:normalize-string() is zero or one; supplied value has cardinality more than one")
	}
	itm := arg[0]
	if str, ok := itm.(string); ok {
		str = multipleWSRegexp.ReplaceAllString(str, " ")
		str = strings.TrimSpace(str)
		return Sequence{str}, nil
	}
	return Sequence{}, nil
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

func fnRound(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := numberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}

	return Sequence{math.Floor(m + 0.5)}, nil
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

func fnStringJoin(ctx *Context, args []Sequence) (Sequence, error) {
	var joiner string
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("Second argument should be a string")
	}
	joiner = args[1][0].(string)
	collection := make([]string, len(args[0]))
	for i, itm := range args[0] {
		collection[i] = itm.(string)
	}
	return Sequence{strings.Join(collection, joiner)}, nil
}

func fnStringLength(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.context
	} else {
		arg = args[0]
	}

	if len(arg) == 0 {
		return Sequence{0}, nil
	}
	str := itemStringvalue(arg[0])
	// todo: non-string and non-element gives error
	return Sequence{utf8.RuneCountInString(str)}, nil
}

func fnTrue(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{true}, nil
}
func fnUppercase(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]

	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if str, ok := arg[0].(string); ok {
		return Sequence{strings.ToUpper(str)}, nil
	}
	return Sequence{""}, nil
}

func init() {
	xpathfunctions = make(map[string]*Function)
	multipleWSRegexp = regexp.MustCompile(`\s+`)
	RegisterFunction(&Function{Name: "abs", Namespace: fnNS, F: fnAbs, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: fnNS, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ceiling", Namespace: fnNS, F: fnCeiling, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "concat", Namespace: fnNS, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "count", Namespace: fnNS, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "empty", Namespace: fnNS, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: fnNS, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "last", Namespace: fnNS, F: fnLast})
	RegisterFunction(&Function{Name: "local-name", Namespace: fnNS, F: fnLocalName, MaxArg: 1})
	RegisterFunction(&Function{Name: "max", Namespace: fnNS, F: fnMax, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "min", Namespace: fnNS, F: fnMin, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: fnNS, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-space", Namespace: fnNS, F: fnNormalizeSpace, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: fnNS, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: fnNS, F: fnPosition})
	RegisterFunction(&Function{Name: "round", Namespace: fnNS, F: fnRound, MaxArg: 1})
	RegisterFunction(&Function{Name: "string", Namespace: fnNS, F: fnString, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-join", Namespace: fnNS, F: fnStringJoin, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-length", Namespace: fnNS, F: fnStringLength, MaxArg: 1})
	RegisterFunction(&Function{Name: "upper-case", Namespace: fnNS, F: fnUppercase, MinArg: 1, MaxArg: 1})
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
	xpathfunctions[f.Namespace+" "+f.Name] = f
}

func getfunction(namespace, name string) *Function {
	return xpathfunctions[namespace+" "+name]
}

func callFunction(name string, arguments []Sequence, ctx *Context) (Sequence, error) {
	parts := strings.Split(name, ":")
	var ns string
	var ok bool
	if len(parts) == 2 {
		if ns, ok = ctx.namespaces[parts[0]]; ok {
			name = parts[1]
		} else {
			return nil, fmt.Errorf("Could not find namespace for prefix %q", parts[0])
		}
	} else {
		ns = fnNS
	}

	fn := getfunction(ns, name)
	if fn == nil {
		return nil, fmt.Errorf("Could not find function %q in namespace %q", name, ns)
	}
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
