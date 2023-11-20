package goxpath

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/speedata/goxml"
)

var (
	xpathfunctions   = make(map[string]*Function)
	multipleWSRegexp *regexp.Regexp
)

// XSDate is a date instance
type XSDate time.Time

func (d XSDate) String() string {
	// for example 2004-05-12+01:00
	return time.Time(d).Format("2006-01-02-07:00")
}

// XSDateTime is a date time instance
type XSDateTime time.Time

func (d XSDateTime) String() string {
	// for example 2004-05-12T18:17:15.125Z
	return time.Time(d).Format("2006-01-02T15:04:05.000-07:00")
}

// XSTime is a time instance
type XSTime time.Time

func (d XSTime) String() string {
	// for example 23:17:00.000-05:00
	return time.Time(d).Format("15:04:05.000-07:00")
}

var currentTimeGetter = func() time.Time {
	return time.Now()
}

const (
	nsFN = "http://www.w3.org/2005/xpath-functions"
	nsXS = "http://www.w3.org/2001/XMLSchema"
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

func fnCodepointEqual(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 || len(args[1]) == 0 {
		return Sequence{}, nil
	}
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{firstarg == secondarg}, nil
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

func fnCompare(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 || len(args[1]) == 0 {
		return Sequence{}, nil
	}
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.Compare(firstarg, secondarg)}, nil
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

func fnCurrentDate(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSDate(currentTimeGetter())}, nil
}

func fnCurrentDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSDateTime(currentTimeGetter())}, nil
}

func fnCurrentTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSTime(currentTimeGetter())}, nil
}

func fnEmpty(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{len(args[0]) == 0}, nil
}

func fnEndsWith(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.HasSuffix(firstarg, secondarg)}, nil

}

func fnFalse(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{false}, nil
}

func fnFloor(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := NumberValue(seq)
	return Sequence{math.Floor(itm)}, err
}

func fnHoursFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of hours-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of hours-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("15")}, nil
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

func fnMinutesFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of minutes-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of minutes-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("04")}, nil
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

func fnSecondsFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of seconds-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of seconds-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("05")}, nil
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

func fnStartsWith(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.HasPrefix(firstarg, secondarg)}, nil

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

func fnSubstringAfter(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	_, after, _ := strings.Cut(firstarg, secondarg)

	return Sequence{after}, nil
}

func fnSubstringBefore(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	before, _, _ := strings.Cut(firstarg, secondarg)

	return Sequence{before}, nil
}

func fnTranslate(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	thirdarg, err := StringValue(args[2])
	if err != nil {
		return nil, err
	}
	var replace []string
	var i int
	var s rune
	var t string
	thirdArgRunes := []rune(thirdarg)
	for i, s = range secondarg {
		if len(thirdArgRunes) > i {
			t = string(thirdArgRunes[i])
		} else {
			t = ""
		}
		replace = append(replace, string(s), t)
	}
	repl := strings.NewReplacer(replace...)

	return Sequence{repl.Replace(firstarg)}, nil
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
	multipleWSRegexp = regexp.MustCompile(`\s+`)
	RegisterFunction(&Function{Name: "abs", Namespace: nsFN, F: fnAbs, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: nsFN, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ceiling", Namespace: nsFN, F: fnCeiling, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "codepoint-equal", Namespace: nsFN, F: fnCodepointEqual, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "codepoints-to-string", Namespace: nsFN, F: fnCodepointsToString, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "compare", Namespace: nsFN, F: fnCompare, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "concat", Namespace: nsFN, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "contains", Namespace: nsFN, F: fnContains, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "count", Namespace: nsFN, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "current-date", Namespace: nsFN, F: fnCurrentDate, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-dateTime", Namespace: nsFN, F: fnCurrentDateTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-time", Namespace: nsFN, F: fnCurrentTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "empty", Namespace: nsFN, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ends-with", Namespace: nsFN, F: fnEndsWith, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "false", Namespace: nsFN, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: nsFN, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-time", Namespace: nsFN, F: fnHoursFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "minutes-from-time", Namespace: nsFN, F: fnMinutesFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "seconds-from-time", Namespace: nsFN, F: fnSecondsFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "last", Namespace: nsFN, F: fnLast})
	RegisterFunction(&Function{Name: "local-name", Namespace: nsFN, F: fnLocalName, MaxArg: 1})
	RegisterFunction(&Function{Name: "lower-case", Namespace: nsFN, F: fnLowercase, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "matches", Namespace: nsFN, F: fnMatches, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "max", Namespace: nsFN, F: fnMax, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "min", Namespace: nsFN, F: fnMin, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: nsFN, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-space", Namespace: nsFN, F: fnNormalizeSpace, MaxArg: 1})
	RegisterFunction(&Function{Name: "number", Namespace: nsFN, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: nsFN, F: fnPosition})
	RegisterFunction(&Function{Name: "replace", Namespace: nsFN, F: fnReplace, MinArg: 3, MaxArg: 4})
	RegisterFunction(&Function{Name: "reverse", Namespace: nsFN, F: fnReverse, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "round", Namespace: nsFN, F: fnRound, MaxArg: 1})
	RegisterFunction(&Function{Name: "string", Namespace: nsFN, F: fnString, MaxArg: 1})
	RegisterFunction(&Function{Name: "starts-with", Namespace: nsFN, F: fnStartsWith, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-join", Namespace: nsFN, F: fnStringJoin, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-length", Namespace: nsFN, F: fnStringLength, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-to-codepoints", Namespace: nsFN, F: fnStringToCodepoints, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "substring", Namespace: nsFN, F: fnSubstring, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-before", Namespace: nsFN, F: fnSubstringBefore, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-after", Namespace: nsFN, F: fnSubstringAfter, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "translate", Namespace: nsFN, F: fnTranslate, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "true", Namespace: nsFN, F: fnTrue})
	RegisterFunction(&Function{Name: "tokenize", Namespace: nsFN, F: fnTokenize, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "upper-case", Namespace: nsFN, F: fnUppercase, MinArg: 1, MaxArg: 1})
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
		ns = nsFN
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
