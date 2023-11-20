package goxpath

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
	itm, err := NumberValue(seq)
	return Sequence{math.Abs(itm)}, err
}

func fnBoolean(ctx *Context, args []Sequence) (Sequence, error) {
	bv, err := BooleanValue(args[0])
	return Sequence{bv}, err
}

func fnCeiling(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := NumberValue(seq)
	return Sequence{math.Ceil(itm)}, err
}

func fnCodepointsToString(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	var sb strings.Builder
	for _, itm := range inputSeq {
		i, err := ToXSInteger(itm)
		if err != nil {
			return nil, err
		}
		sb.WriteRune(rune(i))
	}
	return Sequence{sb.String()}, nil
}

func fnConcat(ctx *Context, args []Sequence) (Sequence, error) {
	var str []string

	for _, seq := range args {
		str = append(str, seq.Stringvalue())
	}
	return Sequence{strings.Join(str, "")}, nil
}

func fnContains(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	testSeq := args[1]
	if len(testSeq) == 0 {
		return Sequence{true}, nil
	}

	if len(inputSeq) == 0 {
		// If len inputSeq == 0, return false unless len testSeq == 0 but len
		// testSeq handled above.
		return Sequence{false}, nil
	}
	var err error
	var inputText, testText string
	if inputText, err = StringValue(inputSeq); err != nil {
		return nil, err
	}
	if testText, err = StringValue(testSeq); err != nil {
		return nil, err
	}

	return Sequence{strings.Contains(inputText, testText)}, nil
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
	itm, err := NumberValue(seq)
	return Sequence{math.Floor(itm)}, err
}

func fnLast(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.size}, nil
}

func fnLocalName(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
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

func fnLowercase(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]

	if len(inputSeq) == 0 {
		return Sequence{""}, nil
	}
	var str string
	var err error
	if str, err = StringValue(inputSeq); err != nil {
		return Sequence{""}, err
	}
	return Sequence{strings.ToLower(str)}, nil
}

func fnMatches(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	regexSeq := args[1]
	if len(inputSeq) == 0 {
		return nil, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}

	if len(regexSeq) == 0 {
		return nil, fmt.Errorf("second argument of fn:matches must be a regular expression")
	}
	regex, err := StringValue(regexSeq)
	if err != nil {
		return nil, err
	}

	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("second argument of fn:matches must be a valid regular expression")
	}

	return Sequence{r.MatchString(input)}, nil
}

func fnMax(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
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
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
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
		arg = ctx.sequence
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
	b, err := BooleanValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{!b}, nil
}

func fnNumber(ctx *Context, args []Sequence) (Sequence, error) {
	nv, err := NumberValue(args[0])
	return Sequence{nv}, err
}

func fnPosition(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.Pos}, nil
}

func fnReplace(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	regexSeq := args[1]
	replaceSeq := args[2]
	if len(inputSeq) == 0 {
		return nil, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}

	if len(regexSeq) == 0 {
		return nil, fmt.Errorf("second argument of fn:replace must be a regular expression")
	}
	regex, err := StringValue(regexSeq)
	if err != nil {
		return nil, err
	}

	rexpr, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("second argument of fn:replace must be a regular expression")

	}

	replace, err := StringValue(replaceSeq)
	if err != nil {
		return nil, err
	}

	// xpath uses $12 for $12 or $1, depending on the existence of $12 or $1.
	// go on the other hand uses $12 for $12 and never for $1, so you have to write
	// $1 as ${1} if there is text after the $1.
	// We escape the $n backwards to prevent expansion of $12 to ${1}2
	for i := rexpr.NumSubexp(); i > 0; i-- {
		// first create rexepx that match "$i"
		x := fmt.Sprintf(`\$(%d)`, i)
		nummatcher := regexp.MustCompile(x)
		replace = nummatcher.ReplaceAllString(replace, fmt.Sprintf(`$${%d}`, i))
	}
	str := rexpr.ReplaceAllString(input, replace)
	return Sequence{str}, nil
}

func fnReverse(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	if len(inputSeq) == 0 {
		return inputSeq, nil
	}
	var retSeq = make(Sequence, len(inputSeq))
	i := 0
	l := len(inputSeq)
	for {
		retSeq[i] = inputSeq[l-i-1]
		i++
		if i >= l {
			break
		}
	}

	return retSeq, nil
}

func fnRound(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}

	return Sequence{math.Floor(m + 0.5)}, nil
}

func fnString(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	sv, err := StringValue(arg)
	if err != nil {
		return nil, err
	}
	return Sequence{sv}, nil
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
		arg = ctx.sequence
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

func fnStringToCodepoints(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	if len(inputSeq) == 0 {
		return Sequence{}, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}
	var retSeq Sequence

	for _, r := range input {
		retSeq = append(retSeq, int(r))
	}
	return retSeq, nil
}

func fnSubstring(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	startSeq := args[1]

	var err error
	var inputText string
	var startNum, lenNum float64
	if inputText, err = StringValue(inputSeq); err != nil {
		return nil, err
	}
	if startNum, err = NumberValue(startSeq); err != nil {
		return nil, err
	}
	inputRunes := []rune(inputText)
	if len(args) > 2 {
		lenSeq := args[2]
		if lenNum, err = NumberValue(lenSeq); err != nil {
			return nil, err
		}
		inputRunes = inputRunes[int(startNum)-1 : int(startNum)+int(lenNum)-1]
		return Sequence{string(inputRunes)}, nil
	}
	return Sequence{string(inputRunes[int(startNum)-1:])}, nil
}

func fnTrue(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{true}, nil
}

func fnUppercase(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]

	if len(arg) == 0 {
		return Sequence{""}, nil
	}

	return Sequence{strings.ToUpper(arg.Stringvalue())}, nil
}

func fnTokenize(ctx *Context, args []Sequence) (Sequence, error) {
	input := args[0]
	if len(input) == 0 {
		return Sequence{}, nil
	}
	regexpSeq := args[1]
	if len(regexpSeq) != 1 {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	var regexpStr string
	var ok bool
	if regexpStr, ok = args[1][0].(string); !ok {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	r, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	text := input.Stringvalue()
	idx := r.FindAllStringIndex(text, -1)

	pos := 0
	var res []string
	for _, v := range idx {
		res = append(res, text[pos:v[0]])
		pos = v[1]
	}
	res = append(res, text[pos:])

	var retSeq Sequence
	for _, str := range res {
		retSeq = append(retSeq, str)
	}
	return retSeq, nil
}

func init() {
	xpathfunctions = make(map[string]*Function)
	multipleWSRegexp = regexp.MustCompile(`\s+`)
	RegisterFunction(&Function{Name: "abs", Namespace: fnNS, F: fnAbs, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: fnNS, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ceiling", Namespace: fnNS, F: fnCeiling, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "codepoints-to-string", Namespace: fnNS, F: fnCodepointsToString, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "concat", Namespace: fnNS, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "contains", Namespace: fnNS, F: fnContains, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "count", Namespace: fnNS, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "empty", Namespace: fnNS, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: fnNS, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "last", Namespace: fnNS, F: fnLast})
	RegisterFunction(&Function{Name: "local-name", Namespace: fnNS, F: fnLocalName, MaxArg: 1})
	RegisterFunction(&Function{Name: "lower-case", Namespace: fnNS, F: fnLowercase, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "matches", Namespace: fnNS, F: fnMatches, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "max", Namespace: fnNS, F: fnMax, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "min", Namespace: fnNS, F: fnMin, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: fnNS, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-space", Namespace: fnNS, F: fnNormalizeSpace, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: fnNS, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: fnNS, F: fnPosition})
	RegisterFunction(&Function{Name: "replace", Namespace: fnNS, F: fnReplace, MinArg: 3, MaxArg: 4})
	RegisterFunction(&Function{Name: "reverse", Namespace: fnNS, F: fnReverse, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "round", Namespace: fnNS, F: fnRound, MaxArg: 1})
	RegisterFunction(&Function{Name: "string", Namespace: fnNS, F: fnString, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-join", Namespace: fnNS, F: fnStringJoin, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-length", Namespace: fnNS, F: fnStringLength, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-to-codepoints", Namespace: fnNS, F: fnStringToCodepoints, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "substring", Namespace: fnNS, F: fnSubstring, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "true", Namespace: fnNS, F: fnTrue})
	RegisterFunction(&Function{Name: "tokenize", Namespace: fnNS, F: fnTokenize, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "upper-case", Namespace: fnNS, F: fnUppercase, MinArg: 1, MaxArg: 1})
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
		if ns, ok = ctx.Namespaces[parts[0]]; ok {
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
