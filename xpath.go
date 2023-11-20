package goxpath

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/speedata/goxml"
	"golang.org/x/net/html"
)

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

// Context is needed for variables, namespaces and XML navigation.
type Context struct {
	Namespaces   map[string]string           // Storage for (private) name spaces
	Store        map[interface{}]interface{} // Store can be used for private variables accessible in functions
	Pos          int                         // Used to determine the position() in the sequence
	vars         map[string]Sequence
	sequence     Sequence
	ctxPositions []int
	ctxLengths   []int
	size         int
	xmldoc       *goxml.XMLDocument
}

// NewContext returns a context from the xml document
func NewContext(doc *goxml.XMLDocument) *Context {
	ctx := &Context{
		xmldoc:     doc,
		vars:       make(map[string]Sequence),
		Namespaces: make(map[string]string),
	}
	ctx.Namespaces["fn"] = fnNS
	return ctx
}

// CopyContext creates a new context with the underlying xml document but can be
// changed without changing the original context.
func CopyContext(cur *Context) *Context {
	ctx := &Context{
		xmldoc:     cur.xmldoc,
		vars:       make(map[string]Sequence),
		Namespaces: make(map[string]string),
		Store:      make(map[interface{}]interface{}),
		sequence:   cur.sequence,
	}

	for k, v := range cur.vars {
		ctx.vars[k] = v
	}

	for k, v := range cur.Namespaces {
		ctx.Namespaces[k] = v
	}
	for k, v := range cur.Store {
		ctx.Store[k] = v
	}
	ctx.Pos = cur.Pos
	for _, l := range cur.ctxLengths {
		ctx.ctxLengths = append(ctx.ctxLengths, l)
	}
	for _, l := range cur.ctxPositions {
		ctx.ctxLengths = append(ctx.ctxPositions, l)
	}
	return ctx
}

// SetContextSequence sets the context sequence and returns the previous one.
func (ctx *Context) SetContextSequence(seq Sequence) Sequence {
	oldCtx := ctx.sequence
	ctx.sequence = seq
	return oldCtx
}

// GetContextSequence returns the current context.
func (ctx *Context) GetContextSequence() Sequence {
	return ctx.sequence
}

// Document moves the node navigator to the document and retuns it
func (ctx *Context) Document() goxml.XMLNode {
	ctx.sequence = Sequence{ctx.xmldoc}
	ctx.ctxPositions = nil
	ctx.ctxLengths = nil
	return ctx.xmldoc
}

// Root moves the node navigator to the root node of the document
func (ctx *Context) Root() (Sequence, error) {
	var err error
	cur, err := ctx.xmldoc.Root()
	if err != nil {
		return nil, err
	}
	ctx.sequence = Sequence{cur}
	ctx.ctxPositions = nil
	ctx.ctxLengths = nil
	return ctx.sequence, err
}

type testFunc func(Item) bool
type testfuncChildren func(*goxml.Element) bool
type testfuncAttributes func(*goxml.Attribute) bool

// Current returns all elements in the context that satisfy the testfunc.
func (ctx *Context) Current(tf testfuncChildren) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	ctx.ctxLengths = []int{}
	pos := 0
	l := 0
	for _, n := range ctx.sequence {
		if elt, ok := n.(*goxml.Element); ok {
			if tf(elt) {
				pos++
				l++
				ctx.ctxPositions = append(ctx.ctxPositions, pos)
				seq = append(seq, n)
			}
		}
	}
	for i := 0; i < l; i++ {
		ctx.ctxLengths = append(ctx.ctxLengths, l)
	}

	ctx.sequence = seq
	return seq, nil
}

// Attributes returns all attributes of the current node that satisfy the testfunc
func (ctx *Context) Attributes(tf testfuncAttributes) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	for _, n := range ctx.sequence {
		if attr, ok := n.(*goxml.Attribute); ok {
			if tf(attr) {
				ctx.ctxPositions = append(ctx.ctxPositions, 1)
				seq = append(seq, attr)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func isElement(itm Item) bool {
	if _, ok := itm.(*goxml.Element); ok {
		return true
	}
	return false
}

func isNode(itm Item) bool {
	return true
}

func isAttribute(itm Item) bool {
	if _, ok := itm.(*goxml.Attribute); ok {
		return true
	}
	return false
}

func returnAttributeNameTest(name string) func(Item) bool {
	return func(itm Item) bool {
		if attr, ok := itm.(*goxml.Attribute); ok {
			if attr.Name == name {
				return true
			}
		}
		return false
	}
}

func returnElementNameTest(name string) func(Item) bool {
	return func(itm Item) bool {
		if elt, ok := itm.(*goxml.Element); ok {
			if elt.Name == name {
				return true
			}
		}
		return false
	}
}

// Filter applies predicates to the context
func (ctx *Context) Filter(filter EvalFunc) (Sequence, error) {
	var result Sequence
	var lengths []int
	var positions []int
	if ctx.ctxPositions != nil {
		positions = ctx.ctxPositions
		lengths = ctx.ctxLengths
	} else {
		positions = make([]int, len(ctx.sequence))
		lengths = make([]int, len(ctx.sequence))
		for i := 0; i < len(ctx.sequence); i++ {
			positions[i] = i + 1
			lengths[i] = 1
		}
	}

	copyContext := ctx.sequence
	predicate, err := filter(ctx)
	if err != nil {
		return nil, err
	}

	// [1] is the same as "position() = 1"
	if len(predicate) == 1 {
		var predicateIsNum bool
		var predicateNum int
		if p0, ok := predicate[0].(float64); ok {
			predicateIsNum = true
			predicateNum = int(p0)
		} else if p0, ok := predicate[0].(int); ok {
			predicateIsNum = true
			predicateNum = p0
		}
		if predicateIsNum {
			var seq Sequence
			for i, itm := range ctx.sequence {
				pos := positions[i]
				if predicateNum == pos {
					seq = append(seq, itm)
				}
			}
			ctx.sequence = seq
			return seq, nil
		}
	}

	for i, itm := range copyContext {
		ctx.sequence = Sequence{itm}
		ctx.Pos = positions[i]
		if len(lengths) > i {
			ctx.size = lengths[i]
		} else {
			ctx.size = 1
		}
		predicate, err := filter(ctx)
		if err != nil {
			return nil, err
		}
		evalItem, err := BooleanValue(predicate)
		if err != nil {
			return nil, err
		}
		if evalItem {
			result = append(result, itm)
		}
	}
	ctx.size = len(result)
	if len(result) == 0 {
		result = Sequence{}
	}
	ctx.sequence = result
	return result, nil
}

func returnIsNameTF(name string) testfuncChildren {
	tf := func(elt *goxml.Element) bool {
		if name == "*" || elt.Name == name {
			return true
		}
		return false
	}
	return tf
}

func returnIsNameTFAttr(name string) testfuncAttributes {
	tf := func(elt *goxml.Attribute) bool {
		if name == "*" || elt.Name == name {
			return true
		}
		return false
	}
	return tf
}

// An Item can hold anything such as a number, a string or a node.
type Item interface{}

func itemStringvalue(itm Item) string {
	var ret string
	switch t := itm.(type) {
	case float64:
		ret = fmt.Sprintf("%f", t)
	case int:
		ret = fmt.Sprintf("%d", t)
	case []uint8:
		ret = fmt.Sprintf("%s", t)
	case *goxml.Attribute:
		ret = fmt.Sprintf(t.Value)
	case *goxml.Element:
		ret = fmt.Sprint(t.Stringvalue())
	case goxml.CharData:
		ret = t.Contents
	case []goxml.XMLNode:
		var str strings.Builder
		for _, n := range t {
			str.WriteString(itemStringvalue(n))
		}
		ret = str.String()
	case string:
		ret = t
	case *html.Node:
		var buf strings.Builder
		html.Render(&buf, t)
		ret = buf.String()
	default:
		ret = fmt.Sprint(t)
	}
	return ret
}

// A Sequence is a list of Items
type Sequence []Item

func (s Sequence) String() string {
	var sb strings.Builder
	sb.WriteString(`( `)
	for _, itm := range s {
		fmt.Fprintf(&sb, "%v ", itm)
	}
	sb.WriteString(`)`)
	return sb.String()
}

// Stringvalue returns the concatenation of the string value of each item.
func (s Sequence) Stringvalue() string {
	var sb strings.Builder
	for _, itm := range s {
		sb.WriteString(itemStringvalue(itm))
	}
	return sb.String()
}

// IntValue returns the sequence value as an integer.
func (s Sequence) IntValue() (int, error) {
	if len(s) > 1 {
		return 0, fmt.Errorf("at most one item expected in the sequence")
	}
	if len(s) == 0 {
		return 0, nil
	}
	numberF, err := strconv.ParseFloat(itemStringvalue(s[0]), 64)
	if err != nil {
		return 0, err
	}
	return int(numberF), nil
}

// EvalFunc returns a sequence evaluating the XPath expression in the given
// context.
type EvalFunc func(*Context) (Sequence, error)

func isEqual(a, b interface{}) (bool, error) {
	return a == b, nil
}

func isLessFloat(a, b float64) bool {
	return a < b
}

func doCompareString(op string, a, b string) (bool, error) {
	switch op {
	case "<":
		return a < b, nil
	case "=":
		return a == b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<=":
		return a <= b, nil
	case "!=":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

func doCompareFloat(op string, a, b float64) (bool, error) {
	switch op {
	case "<":
		return a < b, nil
	case "=":
		return a == b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<=":
		return a <= b, nil
	case "!=":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

func doCompareInt(op string, a, b int) (bool, error) {
	switch op {
	case "<":
		return a < b, nil
	case "=":
		return a == b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<=":
		return a <= b, nil
	case "!=":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

type datatype int

const (
	xUnknown datatype = iota
	xDouble
	xInteger
	xString
)

func compareFunc(op string, a, b interface{}) (bool, error) {
	var floatLeft, floatRight float64
	var intLeft, intRight int
	var int64Left, int64Right int64
	var stringLeft, stringRight string
	var dtLeft, dtRight datatype

	var ok bool
	if floatLeft, ok = a.(float64); ok {
		dtLeft = xDouble
	}
	if floatRight, ok = b.(float64); ok {
		dtRight = xDouble
	}
	if intLeft, ok = a.(int); ok {
		dtLeft = xInteger
	}
	if int64Left, ok = a.(int64); ok {
		intLeft = int(int64Left)
		dtLeft = xInteger
	}
	if intRight, ok = b.(int); ok {
		dtRight = xInteger
	}
	if int64Right, ok = b.(int64); ok {
		intRight = int(int64Right)
		dtRight = xInteger
	}
	if stringLeft, ok = a.(string); ok {
		dtLeft = xString
	}
	if stringRight, ok = b.(string); ok {
		dtRight = xString
	}
	if attLeft, ok := a.(*goxml.Attribute); ok {
		dtLeft = xString
		stringLeft = attLeft.Stringvalue()
	}
	if attRight, ok := b.(*goxml.Attribute); ok {
		dtRight = xString
		stringRight = attRight.Stringvalue()
	}

	if dtLeft == xDouble && dtRight == xDouble {
		return doCompareFloat(op, floatLeft, floatRight)
	}
	if dtLeft == xInteger && dtRight == xInteger {
		return doCompareInt(op, intLeft, intRight)
	}
	if dtLeft == xDouble && dtRight == xInteger {
		return doCompareInt(op, int(floatLeft), intRight)
	}
	if dtLeft == xInteger && dtRight == xDouble {
		return doCompareInt(op, intLeft, int(floatRight))
	}
	if dtLeft == xString && dtRight == xString {
		return doCompareString(op, stringLeft, stringRight)
	}
	if dtLeft == xDouble && dtRight == xString {
		var err error
		floatRight, err = strconv.ParseFloat(stringRight, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, floatLeft, floatRight)
	}
	if dtLeft == xString && dtRight == xDouble {
		var err error
		floatLeft, err = strconv.ParseFloat(stringLeft, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, floatLeft, floatRight)
	}

	return false, fmt.Errorf("FORG0001")
}

func doCompare(op string, lhs EvalFunc, rhs EvalFunc) (EvalFunc, error) {
	f := func(ctx *Context) (Sequence, error) {
		left, err := lhs(ctx)
		if err != nil {
			return nil, err
		}
		right, err := rhs(ctx)
		if err != nil {
			return nil, err
		}
		for _, leftitem := range left {
			for _, rightitem := range right {
				ok, err := compareFunc(op, leftitem, rightitem)
				if err != nil {
					return nil, err
				}
				if ok {
					return Sequence{true}, nil
				}
			}
		}
		return Sequence{false}, nil
	}
	return f, nil
}

func doCompareNode(op string, lhs EvalFunc, rhs EvalFunc) (EvalFunc, error) {
	f := func(ctx *Context) (Sequence, error) {
		left, err := lhs(ctx)
		if err != nil {
			return nil, err
		}
		right, err := rhs(ctx)
		if err != nil {
			return nil, err
		}
		if len(left) == 0 || len(right) == 0 {
			return Sequence{}, nil
		}
		if len(left) > 1 {
			return Sequence{}, fmt.Errorf("A sequence of more than one item is not allowed as the first operand of '%s'", op)
		}
		if len(right) > 1 {
			return Sequence{}, fmt.Errorf("A sequence of more than one item is not allowed as the second operand of '%s'", op)
		}
		var leftElement, rightElement *goxml.Element
		if elt, ok := left[0].(*goxml.Element); ok {
			leftElement = elt
		}
		if elt, ok := right[0].(*goxml.Element); ok {
			rightElement = elt
		}

		if op == "is" {
			return Sequence{leftElement.ID == rightElement.ID}, nil
		}
		if op == "<<" {
			return Sequence{leftElement.ID < rightElement.ID}, nil
		}
		if op == ">>" {
			return Sequence{leftElement.ID > rightElement.ID}, nil
		}
		return Sequence{false}, nil
	}
	return f, nil
}

// NumberValue returns the sequence converted to a float.
func NumberValue(s Sequence) (float64, error) {
	if len(s) == 0 {
		return math.NaN(), nil
	}
	if len(s) > 1 {
		return math.NaN(), fmt.Errorf("Required cardinality of first argument of fn:number() is zero or one; supplied value has cardinality more than one")
	}
	firstItem := s[0]
	if num, ok := firstItem.(int); ok {
		return float64(num), nil
	}
	if attr, ok := firstItem.(*goxml.Attribute); ok {
		numberF, err := strconv.ParseFloat(attr.Value, 64)
		if err != nil {
			return 0, err
		}
		return numberF, nil
	}

	if flt, ok := firstItem.(float64); ok {
		return flt, nil
	}
	return math.NaN(), nil
}

// BooleanValue returns the effective boolean value of the sequence.
func BooleanValue(s Sequence) (bool, error) {
	if len(s) == 0 {
		return false, nil
	}
	// if s[0] is a node, return true
	if len(s) == 1 {
		itm := s[0]
		if b, ok := itm.(bool); ok {
			return b, nil
		} else if val, ok := itm.(string); ok {
			return val != "", nil
		} else if val, ok := itm.(float64); ok {
			// val == val false if NaN
			return val != 0 && val == val, nil
		} else if val, ok := itm.(int); ok {
			return val != 0, nil
		} else if isElement(itm) {
			return true, nil
		} else {
			fmt.Printf("itm %#v\n", itm)
		}
	}
	return false, fmt.Errorf("FORG0006 Invalid argument type")
}

// StringValue returns the string value of the sequence by concatenating the
// string values of each item.
func StringValue(s Sequence) (string, error) {
	var sb strings.Builder
	for _, itm := range s {
		sb.WriteString(itemStringvalue(itm))
	}
	return sb.String(), nil
}

// [2] Expr ::= ExprSingle ("," ExprSingle)*
func parseExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "2 parseExpr")
	var efs []EvalFunc
	for {
		ef, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "2 parseExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read() // comma
	}
	if len(efs) == 1 {
		leaveStep(tl, "2 parseExpr (one ExprSingle)")
		return efs[0], nil
	}
	// more than one ExprSingle

	f := func(ctx *Context) (Sequence, error) {
		var ret Sequence
		for _, ef := range efs {
			seq, err := ef(ctx)
			if err != nil {
				return nil, err
			}
			ret = append(ret, seq...)
		}

		return ret, nil
	}
	leaveStep(tl, "2 parseExpr")
	return f, nil
}

// [3] ExprSingle ::= ForExpr | QuantifiedExpr | IfExpr | OrExpr
func parseExprSingle(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "3 parseExprSingle")
	var ef EvalFunc
	var err error
	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"for", "some", "every", "if"}); ok {
		switch op {
		case "for":
			ef, err = parseForExpr(tl)
		case "some", "every":
			tl.unread()
			ef, err = parseQuantifiedExpr(tl)
		case "if":
			leaveStep(tl, "3 parseExprSingle")
			ef, err = parseIfExpr(tl)
		}
		leaveStep(tl, "3 parseExprSingle")
		return ef, err
	}

	ef, err = parseOrExpr(tl)
	if err != nil {
		leaveStep(tl, "3 parseExprSingle (err)")
		return nil, err
	}
	leaveStep(tl, "3 parseExprSingle")
	return ef, nil
}

// [4] ForExpr ::= SimpleForClause "return" ExprSingle
// [5] SimpleForClause ::= "for" "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)*
func parseForExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "4 parseForExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var err error
	var varnames []string

	for {
		vartoken, err := tl.read()
		if err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		if vn, ok := vartoken.Value.(string); ok {
			varnames = append(varnames, vn)
		} else {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, fmt.Errorf("variable name not a string")
		}
		if err = tl.skipNCName("in"); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		if ef, err = parseExprSingle(tl); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if tl.nexttokIsTyp(tokQName) && tl.nexttokIsValue("return") {
			tl.read()
			break
		}
		if err = tl.skipType(tokComma); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
	}

	evalseq, err := parseExprSingle(tl)
	if err != nil {
		leaveStep(tl, "4 parseForExpr (err)")
		return nil, err
	}

	ret := func(ctx *Context) (Sequence, error) {
		var s Sequence

		sequences := []Sequence{}
		for _, ef := range efs {
			newcontext := CopyContext(ctx)
			s, err = ef(newcontext)
			if err != nil {
				leaveStep(tl, "4 parseForExpr (err)")
				return nil, err
			}
			sequences = append(sequences, s)
		}
		// go recursively through all variable combinations
		var f func([]string, []Sequence, *Context) (Sequence, error)
		f = func(varnames []string, sequences []Sequence, ctx *Context) (Sequence, error) {
			seq := Sequence{}
			varname := varnames[0]
			sequence := sequences[0]

			for _, itm := range sequence {
				ctx.vars[varname] = Sequence{itm}
				ctx.sequence = Sequence{itm}

				if len(varnames) > 1 {
					s, err := f(varnames[1:], sequences[1:], ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				} else {
					s, err := evalseq(ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				}
			}
			return seq, nil
		}
		var oldValues []Sequence
		for _, vn := range varnames {
			oldValues = append(oldValues, ctx.vars[vn])
		}
		seq, err := f(varnames, sequences, ctx)
		if err != nil {
			return nil, err
		}
		for i, vn := range varnames {
			ctx.vars[vn] = oldValues[i]
		}
		ctx.sequence = seq
		return seq, nil
	}
	leaveStep(tl, "4 parseForExpr")
	return ret, nil
}

// [6] QuantifiedExpr ::= ("some" | "every") "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)* "satisfies" ExprSingle
func parseQuantifiedExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "6 parseQuantifiedExpr")
	var efs []EvalFunc
	var varnames []string

	someEvery, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"some", "every"}, tokQName)
	if !ok {
		leaveStep(tl, "6 parseQuantifiedExpr (not some/every)")
		return nil, fmt.Errorf("some or every expected, found %q", someEvery)
	}

	for {
		if !tl.nexttokIsTyp(tokVarname) {
			leaveStep(tl, "6 parseQuantifiedExpr (no var name)")
			return nil, fmt.Errorf("%s: variable name expected", someEvery)
		}

		vartok, err := tl.read()
		if err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (err)")
			return nil, err
		}
		varnames = append(varnames, vartok.Value.(string))

		got, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"in"}, tokQName)
		if !ok {
			leaveStep(tl, "6 parseQuantifiedExpr (missing 'in')")
			return nil, fmt.Errorf("'in' expected, got %s", got)
		}

		ef, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)

		_, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"satisfies"}, tokQName)
		if ok {
			break
		}

		if err = tl.skipType(tokComma); err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (missing comma)")
			return nil, err
		}

	}

	var err error
	var lastEf EvalFunc
	lastEf, err = parseExprSingle(tl)
	if err != nil {
		leaveStep(tl, "6 parseQuantifiedExpr (err)")
		return nil, err
	}

	evaler := func(ctx *Context) (Sequence, error) {
		var s Sequence
		sequences := []Sequence{}
		for _, ef := range efs {
			newcontext := CopyContext(ctx)
			s, err = ef(newcontext)
			if err != nil {
				return nil, err
			}
			sequences = append(sequences, s)
		}
		// go recursively through all variable combinations
		var f func([]string, []Sequence, *Context) (Sequence, error)
		f = func(varnames []string, sequences []Sequence, ctx *Context) (Sequence, error) {
			seq := Sequence{}
			varname := varnames[0]
			sequence := sequences[0]

			for _, itm := range sequence {
				ctx.vars[varname] = Sequence{itm}
				ctx.sequence = Sequence{itm}
				if len(varnames) > 1 {
					s, err := f(varnames[1:], sequences[1:], ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				} else {
					s, err := lastEf(ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				}
			}
			return seq, nil
		}
		var oldValues []Sequence
		for _, vn := range varnames {
			oldValues = append(oldValues, ctx.vars[vn])
		}
		seq, err := f(varnames, sequences, ctx)
		if err != nil {
			return nil, err
		}
		for i, vn := range varnames {
			ctx.vars[vn] = oldValues[i]
		}

		if someEvery == "some" {
			for _, itm := range seq {
				bv, err := BooleanValue(Sequence{itm})
				if err != nil {
					return nil, err
				}
				if bv {
					ctx.sequence = Sequence{true}
					goto done
				}
			}
			ctx.sequence = Sequence{false}
		} else {
			for _, itm := range seq {
				bv, err := BooleanValue(Sequence{itm})
				if err != nil {
					return nil, err
				}
				if !bv {
					ctx.sequence = Sequence{false}
					goto done
				}
			}
			ctx.sequence = Sequence{true}
		}
	done:
		return ctx.sequence, nil
	}

	leaveStep(tl, "6 parseQuantifiedExpr")
	return evaler, nil
}

// [7] IfExpr ::= "if" "(" Expr ")" "then" ExprSingle "else" ExprSingle
func parseIfExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "7 parseIfExpr")
	var nexttok *token
	var err error
	var boolEval, thenpart, elsepart EvalFunc

	if err = tl.skipType(tokOpenParen); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, fmt.Errorf("open parenthesis expected, found %v", nexttok.Value)
	}
	if boolEval, err = parseExpr(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if err = tl.skipType(tokCloseParen); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if err = tl.skipNCName("then"); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if thenpart, err = parseExprSingle(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if err = tl.skipNCName("else"); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if elsepart, err = parseExprSingle(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}

	f := func(ctx *Context) (Sequence, error) {
		res, err := boolEval(ctx)
		if err != nil {
			return nil, err
		}
		bv, err := BooleanValue(res)
		if err != nil {
			return nil, err
		}
		if bv {
			return thenpart(ctx)
		}
		return elsepart(ctx)
	}
	leaveStep(tl, "7 parseIfExpr")
	return f, nil
}

// [8] OrExpr ::= AndExpr ( "or" AndExpr )*
func parseOrExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "8 parseOrExpr")
	var efs []EvalFunc
	for {
		ef, err := parseAndExpr(tl)
		if err != nil {
			leaveStep(tl, "8 parseOrExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsValue("or") {
			break
		}
		tl.read()
	}

	if len(efs) == 1 {
		leaveStep(tl, "8 parseOrExpr (#efs = 1)")
		return efs[0], nil
	}
	var ef EvalFunc
	ef = func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := BooleanValue(s)
			if err != nil {
				return nil, err
			}
			if b {
				return Sequence{true}, nil
			}

		}
		return Sequence{false}, nil
	}

	leaveStep(tl, "8 parseOrExpr")
	return ef, nil
}

// [9] AndExpr ::= ComparisonExpr ( "and" ComparisonExpr )*
func parseAndExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "9 parseAndExpr")
	var efs []EvalFunc
	for {
		ef, err := parseComparisonExpr(tl)
		if err != nil {
			leaveStep(tl, "9 parseAndExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsValue("and") {
			break
		}
		tl.read() // and
	}
	if len(efs) == 1 {
		leaveStep(tl, "9 parseAndExpr (#efs == 1)")
		return efs[0], nil
	}

	ef := func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := BooleanValue(s)
			if err != nil {
				return nil, err
			}
			if !b {
				return Sequence{false}, nil
			}

		}
		return Sequence{true}, nil
	}

	leaveStep(tl, "9 parseAndExpr")
	return ef, nil
}

// [10] ComparisonExpr ::= RangeExpr ( (ValueComp | GeneralComp| NodeComp) RangeExpr )?
// [23] ValueComp ::= "eq" | "ne" | "lt" | "le" | "gt" | "ge"
// [22] GeneralComp ::= "=" | "!=" | "<" | "<=" | ">" | ">="
// [24] NodeComp ::= "is" | "<<" | ">>"
func parseComparisonExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "10 parseComparisonExpr")
	var lhs, rhs EvalFunc
	var err error
	if lhs, err = parseRangeExpr(tl); err != nil {
		leaveStep(tl, "10 parseComparisonExpr")
		return nil, err
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"=", "<", ">", "<=", ">=", "!=", "eq", "ne", "lt", "le", "gt", "ge"}); ok {
		if rhs, err = parseRangeExpr(tl); err != nil {
			leaveStep(tl, "10 parseComparisonExpr")
			return nil, err
		}
		leaveStep(tl, "10 parseComparisonExpr")
		return doCompare(op, lhs, rhs)
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"is", "<<", ">>"}); ok {
		if rhs, err = parseRangeExpr(tl); err != nil {
			leaveStep(tl, "10 parseComparisonExpr")
			return nil, err
		}
		leaveStep(tl, "10 parseComparisonExpr")
		return doCompareNode(op, lhs, rhs)
	}

	leaveStep(tl, "10 parseComparisonExpr")
	return lhs, nil
}

// [11] RangeExpr ::= AdditiveExpr ( "to" AdditiveExpr )?
func parseRangeExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "11 parseRangeExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var err error
	for {
		ef, err = parseAdditiveExpr(tl)
		if err != nil {
			leaveStep(tl, "11 parseRangeExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if _, ok := tl.readNexttokIfIsOneOfValue([]string{"to"}); ok {
			// good, just add the next func to the efs slice
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "11 parseRangeExpr (#efs = 1)")
		return efs[0], nil
	}

	retf := func(ctx *Context) (Sequence, error) {
		lhs, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		rhs, err := efs[1](ctx)
		if err != nil {
			return nil, err
		}
		lhsNum, err := NumberValue(lhs)
		if err != nil {
			return nil, err
		}
		rhsNum, err := NumberValue(rhs)
		if err != nil {
			return nil, err
		}
		var seq Sequence
		for i := lhsNum; i <= rhsNum; i++ {
			seq = append(seq, i)
		}
		return seq, nil
	}
	leaveStep(tl, "11 parseRangeExpr")
	return retf, nil
}

// [12] AdditiveExpr ::= MultiplicativeExpr ( ("+" | "-") MultiplicativeExpr )*
func parseAdditiveExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "12 parseAdditiveExpr")
	var efs []EvalFunc
	var operator []string
	var ef EvalFunc
	var err error
	for {
		ef, err = parseMultiplicativeExpr(tl)
		if err != nil {
			leaveStep(tl, "12 parseAdditiveExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"+", "-"}); ok {
			operator = append(operator, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "12 parseAdditiveExpr")
		return efs[0], nil
	}
	ef = func(ctx *Context) (Sequence, error) {
		s, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		sum, err := NumberValue(s)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(efs); i++ {
			s, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			flt, err := NumberValue(s)
			if operator[i-1] == "+" {
				sum += flt
			} else {
				sum -= flt
			}
		}
		return Sequence{sum}, nil
	}
	leaveStep(tl, "12 parseAdditiveExpr")
	return ef, nil
}

// [13] MultiplicativeExpr ::=  UnionExpr ( ("*" | "div" | "idiv" | "mod") UnionExpr )*
func parseMultiplicativeExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "13 parseMultiplicativeExpr")

	var efs []EvalFunc
	var operator []string
	var ef EvalFunc
	var err error
	for {
		ef, err = parseUnionExpr(tl)
		if err != nil {
			leaveStep(tl, "13 parseMultiplicativeExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"*", "div", "idiv", "mod"}); ok {
			operator = append(operator, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "13 parseMultiplicativeExpr")
		return efs[0], nil
	}

	ef = func(ctx *Context) (Sequence, error) {
		s, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		sum, err := NumberValue(s)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(efs); i++ {
			s, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			flt, err := NumberValue(s)
			switch operator[i-1] {
			case "*":
				sum *= flt
			case "div":
				sum /= flt
			case "idiv":
				sum = float64(int(sum / flt))
			case "mod":
				sum = math.Mod(sum, flt)
			}
		}
		return Sequence{sum}, nil
	}

	leaveStep(tl, "13 parseMultiplicativeExpr")
	return ef, nil
}

// [14] UnionExpr ::= IntersectExceptExpr ( ("union" | "|") IntersectExceptExpr )*
func parseUnionExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "14 parseUnionExpr")
	var efs []EvalFunc

	for {
		ef, err := parseIntersectExceptExpr(tl)
		if err != nil {
			leaveStep(tl, "14 parseUnionExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if _, found := tl.readNexttokIfIsOneOfValue([]string{"union", "|"}); !found {
			break
		}
	}

	if len(efs) == 1 {
		leaveStep(tl, "14 parseUnionExpr")
		return efs[0], nil
	}

	ret := func(ctx *Context) (Sequence, error) {
		if len(efs) == 1 {
			return efs[0](ctx)
		}
		var seq Sequence
		for _, ef := range efs {
			efSeq, err := ef(ctx)
			if err != nil {
				return nil, err
			}
			seq = append(seq, efSeq...)
		}
		var nodes goxml.SortByDocumentOrder
		for _, itm := range seq {
			if n, ok := itm.(goxml.XMLNode); ok {
				nodes = append(nodes, n)
			} else {
				fmt.Printf("14: unknown type %T\n", itm)
			}
		}
		// document order
		nodes = nodes.SortAndEliminateDuplicates()
		var retSeq Sequence
		for _, itm := range nodes {
			retSeq = append(retSeq, itm)
		}
		return retSeq, nil
	}

	leaveStep(tl, "14 parseUnionExpr")
	return ret, nil
}

// [15] IntersectExceptExpr  ::= InstanceofExpr ( ("intersect" | "except") InstanceofExpr )*
func parseIntersectExceptExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "15 parseIntersectExceptExpr")
	var efs []EvalFunc
	var intersectExcepts []string
	var err error
	var ef EvalFunc
	if ef, err = parseInstanceofExpr(tl); err != nil {
		leaveStep(tl, "15 parseIntersectExceptExpr")
		return nil, err
	}
	efs = append(efs, ef)
	for {
		var intersectExcept string
		var ok bool
		if intersectExcept, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"intersect", "except"}, tokQName); !ok {
			break
		}
		intersectExcepts = append(intersectExcepts, intersectExcept)
		if ef, err = parseInstanceofExpr(tl); err != nil {
			leaveStep(tl, "15 parseIntersectExceptExpr")
			return nil, err
		}
		efs = append(efs, ef)
	}
	if len(efs) == 1 {
		leaveStep(tl, "15 parseIntersectExceptExpr")
		return efs[0], nil

	}
	evaler := func(ctx *Context) (Sequence, error) {
		ret := Sequence{}
		var left Sequence
		for i, ef := range efs {
			newcontext := CopyContext(ctx)
			right, err := ef(newcontext)
			if err != nil {
				return nil, err
			}

			var lelt, relt *goxml.Element
			var ok, inRight bool
			if i > 0 {
				shouldBeInRight := intersectExcepts[i-1] == "intersect"
				ids := map[int]bool{}
				for _, rItem := range right {
					if relt, ok = rItem.(*goxml.Element); !ok {
						return nil, fmt.Errorf("FIXME: not an element")
					}
					ids[relt.ID] = true
				}
				for _, lItem := range left {
					if lelt, ok = lItem.(*goxml.Element); !ok {
						return nil, fmt.Errorf("FIXME: not an element")
					}
					if _, inRight = ids[lelt.ID]; inRight == shouldBeInRight {
						ret = append(ret, lelt)
					}
				}
			}
			left = right
		}
		ctx.sequence = ret
		return ret, nil
	}
	leaveStep(tl, "15 parseIntersectExceptExpr")
	return evaler, nil
}

// [16] InstanceofExpr ::= TreatExpr ( "instance" "of" SequenceType )?
func parseInstanceofExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "16 parseInstanceofExpr")
	var ef EvalFunc
	var tf testFunc
	var err error
	if ef, err = parseTreatExpr(tl); err != nil {
		leaveStep(tl, "16 parseInstanceofExpr")
		return nil, err
	}

	if tl.nexttokIsValue("instance") {
		tl.read()
		if !tl.nexttokIsValue("of") {
			tl.unread()
			leaveStep(tl, "16 parseInstanceofExpr")
			return ef, nil
		}
		tl.read()

		if tl.readIfTokenFollow([]token{{"empty-sequence", tokQName}, {'(', tokOpenParen}, {')', tokCloseParen}}) {
			evaler := func(ctx *Context) (Sequence, error) {
				seq, err := ef(ctx)
				if err != nil {
					return nil, err
				}
				return Sequence{len(seq) == 0}, nil
			}
			return evaler, nil
		}

		if tf, err = parseSequenceType(tl); err != nil {
			leaveStep(tl, "16 parseInstanceofExpr")
			return nil, err
		}
		var oi string
		oi, _ = tl.readNexttokIfIsOneOfValue([]string{"*", "+", "?"})
		inOfExpr := func(ctx *Context) (Sequence, error) {
			_, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			if oi == "" && len(ctx.sequence) != 1 {
				return Sequence{false}, nil
			}
			if oi == "+" && len(ctx.sequence) < 1 {
				return Sequence{false}, nil
			}
			if oi == "?" && len(ctx.sequence) > 1 {
				return Sequence{false}, nil
			}

			for _, itm := range ctx.sequence {
				if !tf(itm) {
					return Sequence{false}, nil
				}
			}

			return Sequence{true}, nil
		}
		leaveStep(tl, "16 parseInstanceofExpr")
		return inOfExpr, nil
	}

	leaveStep(tl, "16 parseInstanceofExpr")
	return ef, nil
}

// [17] TreatExpr ::= CastableExpr ( "treat" "as" SequenceType )?
func parseTreatExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "17 parseTreatExpr")
	var ef EvalFunc
	ef, err := parseCastableExpr(tl)
	if err != nil {
		leaveStep(tl, "17 parseTreatExpr")
		return nil, err
	}
	var ok bool
	if _, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"treat"}, tokQName); ok {
		if err = tl.skipNCName("as"); err != nil {
			leaveStep(tl, "17 parseTreatExpr")
			return nil, err
		}

		if tl.readIfTokenFollow([]token{{"empty-sequence", tokQName}, {'(', tokOpenParen}, {')', tokCloseParen}}) {
			evaler := func(ctx *Context) (Sequence, error) {
				seq, err := ef(ctx)
				if err != nil {
					return nil, err
				}
				return Sequence{len(seq) == 0}, nil
			}
			return evaler, nil
		}

		_, err := parseSequenceType(tl)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("treat as not implemented")
	}

	leaveStep(tl, "17 parseTreatExpr")
	return ef, nil
}

// [18] CastableExpr ::= CastExpr ( "castable" "as" SingleType )?
func parseCastableExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "18 parseCastableExpr")
	var ef EvalFunc
	ef, err := parseCastExpr(tl)
	if err != nil {
		leaveStep(tl, "18 parseCastableExpr")
		return nil, err
	}

	var ok bool
	if _, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"castable"}, tokQName); ok {
		if err = tl.skipNCName("as"); err != nil {
			leaveStep(tl, "18 parseCastableExpr")
			return nil, err
		}
		if err = tl.skipType(tokQName); err != nil {
			leaveStep(tl, "18 parseCastableExpr")
			return nil, err
		}
		tl.readNexttokIfIsOneOfValueAndType([]string{"?"}, tokOperator)
		return nil, fmt.Errorf("castable as not implemented")
	}

	leaveStep(tl, "18 parseCastableExpr")
	return ef, nil
}

// [19] CastExpr ::= UnaryExpr ( "cast" "as" SingleType )?
func parseCastExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "19 parseCastExpr")
	var ef EvalFunc
	ef, err := parseUnaryExpr(tl)
	if err != nil {
		leaveStep(tl, "19 parseCastExpr")
		return nil, err
	}

	leaveStep(tl, "19 parseCastExpr")
	return ef, nil
}

// [20] UnaryExpr ::= ("-" | "+")* ValueExpr
func parseUnaryExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "20 parseUnaryExpr")
	var hasOP bool
	mult := 1
	for {
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"+", "-"}); ok {
			hasOP = true
			if op == "-" {
				mult *= -1
			}
		} else {
			break
		}
	}
	pv, err := parseValueExpr(tl)
	if err != nil {
		leaveStep(tl, "20 parseUnaryExpr")
		return nil, err
	}

	if !hasOP {
		leaveStep(tl, "20 parseUnaryExpr")
		return pv, nil
	}
	var ef EvalFunc
	ef = func(ctx *Context) (Sequence, error) {
		if mult == -1 {
			seq, err := pv(ctx)
			if err != nil {
				return nil, err
			}
			flt, err := NumberValue(seq)
			if err != nil {
				return nil, err
			}
			return Sequence{flt * -1}, nil
		}
		return pv(ctx)
	}

	leaveStep(tl, "20 parseUnaryExpr")
	return ef, nil
}

// [21] ValueExpr ::= PathExpr
func parseValueExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "21 parseValueExpr")
	var ef EvalFunc
	ef, err := parsePathExpr(tl)
	if err != nil {
		leaveStep(tl, "21 parseValueExpr (err)")
		return nil, err
	}

	leaveStep(tl, "21 parseValueExpr")
	return ef, nil
}

// [25] PathExpr ::= ("/" RelativePathExpr?) | ("//" RelativePathExpr) | RelativePathExpr
func parsePathExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "25 parsePathExpr")
	var rpe EvalFunc
	var op string
	var hasOP bool
	op, hasOP = tl.readNexttokIfIsOneOfValue([]string{"/", "//"})

	rpe, err := parseRelativePathExpr(tl)
	if err != nil {
		leaveStep(tl, "25 parsePathExpr (err)")
		return nil, err
	}

	if hasOP {
		fn := func(ctx *Context) (Sequence, error) {
			ctx.Document()
			if op == "//" {
				ctx.descendantOrSelfAxis(isNode)
			}
			seq, err := rpe(ctx)
			if err != nil {
				return nil, err
			}
			return seq, nil
		}
		return fn, nil
	}

	leaveStep(tl, "25 parsePathExpr")
	return rpe, nil
}

// [26] RelativePathExpr ::= StepExpr (("/" | "//") StepExpr)*
func parseRelativePathExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "26 parseRelativePathExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var ops []string
	var err error

	for {
		ef, err := parseStepExpr(tl)
		if err != nil {
			leaveStep(tl, "26 parseRelativePathExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"/", "//"}); ok {
			ops = append(ops, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "26 parseRelativePathExpr (1)")
		return efs[0], nil // just a simple StepExpr
	}

	ef = func(ctx *Context) (Sequence, error) {
		var retseq Sequence
		var seq Sequence
		for i := 0; i < len(efs); i++ {
			ef := efs[i]
			retseq = retseq[:0]
			if len(ctx.sequence) == 0 {
				if seq, err = ef(ctx); err != nil {
					return nil, err
				}
				retseq = append(retseq, seq...)
			} else {
				copyContext := ctx.sequence
				ctx.size = len(copyContext)
				for j, itm := range copyContext {
					ctx.sequence = Sequence{itm}
					ctx.Pos = j + 1
					if seq, err = ef(ctx); err != nil {
						return nil, err
					}
					retseq = append(retseq, seq...)
				}
			}
			ctx.sequence = ctx.sequence[:0]
			for _, itm := range retseq {
				ctx.sequence = append(ctx.sequence, itm)
			}

			if i < len(ops) && ops[i] == "//" {
				ctx.descendantOrSelfAxis(isElement)
				retseq = append(retseq, ctx.sequence...)
			}
		}

		return retseq, nil
	}

	leaveStep(tl, "26 parseRelativePathExpr")
	return ef, nil
}

// [27] StepExpr := FilterExpr | AxisStep
func parseStepExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "27 parseStepExpr")
	var ef EvalFunc
	ef, err := parseFilterExpr(tl)
	if err != nil {
		leaveStep(tl, "27 parseStepExpr (err1)")
		return nil, err
	}
	if ef == nil {
		ef, err = parseAxisStep(tl)
	}
	if err != nil {
		leaveStep(tl, "27 parseStepExpr (err)")
		return nil, err
	}

	if ef == nil {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{}, nil
		}
	}
	leaveStep(tl, "27 parseStepExpr")
	return ef, nil
}

// [28] AxisStep ::= (ReverseStep | ForwardStep) PredicateList
// [39] PredicateList ::= Predicate*
func parseAxisStep(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "28 parseAxisStep")
	var ef EvalFunc
	var err error
	var predicates []EvalFunc
	if ef, err = parseForwardStep(tl); err != nil {
		leaveStep(tl, "28 parseAxisStep (err)")
		return nil, err
	}
	for {
		if tl.nexttokIsTyp(tokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				leaveStep(tl, "28 parseAxisStep (err)")
				return nil, err
			}
			predicates = append(predicates, predicate)
			err = tl.skipType(tokCloseBracket)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	if len(predicates) == 0 {
		leaveStep(tl, "28 parseAxisStep (b)")
		return ef, nil
	}
	ff := func(ctx *Context) (Sequence, error) {
		_, err = ef(ctx)
		if err != nil {
			return nil, err
		}
		for _, predicate := range predicates {
			_, err = ctx.Filter(predicate)
			if err != nil {
				return nil, err
			}
			ctx.size = len(ctx.sequence)
		}
		return ctx.sequence, nil
	}

	leaveStep(tl, "28 parseAxisStep (b)")
	return ff, nil
}

type axis int

const (
	axisChild axis = iota
	axisSelf
	axisDescendant
	axisDescendantOrSelf
	axisFollowing
	axisFollowingSibling
	axisParent
	axisAncestor
	axisAncestorOrSelf
	axisPreceding
	axisPrecedingSibling
)

func (a axis) String() string {
	switch a {
	case axisChild:
		return "child"
	case axisSelf:
		return "self"
	case axisDescendant:
		return "descendant"
	case axisDescendantOrSelf:
		return "descendant-or-self"
	case axisFollowing:
		return "following"
	case axisFollowingSibling:
		return "following-sibling"
	case axisParent:
		return "parent"
	case axisAncestor:
		return "ancestor"
	case axisAncestorOrSelf:
		return "ancestor-or-self"
	case axisPreceding:
		return "preceding"
	case axisPrecedingSibling:
		return "preceding-sibling"

	}
	return ""
}

// [29] ForwardStep ::= (ForwardAxis NodeTest) | AbbrevForwardStep
// [31] AbbrevForwardStep ::= "@"? NodeTest
func parseForwardStep(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "29 parseForwardStep")
	var err error

	stepAxis := axisChild
	tl.attributeMode = false

	if tl.nexttokIsTyp(tokDoubleColon) {
		nexttok, err := tl.read()
		if err != nil {
			return nil, err
		}

		switch nexttok.Value.(string) {
		case "attribute":
			tl.attributeMode = true
		case "child":
			stepAxis = axisChild
		case "self":
			stepAxis = axisSelf
		case "descendant":
			stepAxis = axisDescendant
		case "descendant-or-self":
			stepAxis = axisDescendantOrSelf
		case "following":
			stepAxis = axisFollowing
		case "following-sibling":
			stepAxis = axisFollowingSibling
		case "parent":
			stepAxis = axisParent
		case "ancestor":
			stepAxis = axisAncestor
		case "ancestor-or-self":
			stepAxis = axisAncestorOrSelf
		case "preceding-sibling":
			stepAxis = axisPrecedingSibling
		case "preceding":
			stepAxis = axisPreceding
		default:
			return nil, fmt.Errorf("unknown axis %s", nexttok.Value.(string))
		}
	}
	if tl.nexttokIsValue("..") && tl.nexttokIsTyp(tokOperator) {
		tl.read()
		ef := func(ctx *Context) (Sequence, error) {
			return ctx.parentAxis(isNode)
		}
		leaveStep(tl, "29 parseForwardStep (..)")
		return ef, nil
	}

	if tl.nexttokIsValue("@") {
		tl.read() // @
		tl.attributeMode = true
	}
	var tf testFunc
	if tf, err = parseNodeTest(tl); err != nil {
		leaveStep(tl, "29 parseForwardStep (err)")
		return nil, err
	}
	if tf == nil {
		leaveStep(tl, "29 parseForwardStep (nil)")
		return nil, nil
	}
	ret := func(ctx *Context) (Sequence, error) {
		var ret Sequence
		switch stepAxis {
		case axisSelf:
			// nothing
		case axisChild:
			_, err = ctx.childAxis(tf)
		case axisDescendant:
			_, err = ctx.descendantAxis(tf)
		case axisDescendantOrSelf:
			_, err = ctx.descendantOrSelfAxis(tf)
		case axisFollowing:
			_, err = ctx.followingAxis(tf)
		case axisFollowingSibling:
			_, err = ctx.followingSiblingAxis(tf)
		case axisParent:
			_, err = ctx.parentAxis(tf)
		case axisAncestor:
			_, err = ctx.ancestorAxis(tf)
		case axisAncestorOrSelf:
			_, err = ctx.ancestorOrSelfAxis(tf)
		case axisPrecedingSibling:
			_, err = ctx.precedingSiblingAxis(tf)
		case axisPreceding:
			_, err = ctx.precedingAxis(tf)
		default:
			return nil, fmt.Errorf("unknown axis %s", stepAxis)
		}
		if err != nil {
			return nil, err
		}
		copyContext := ctx.sequence
		for _, itm := range copyContext {
			ret = append(ret, itm)
		}
		ctx.sequence = ret
		ctx.size = len(ret)
		return ret, nil
	}

	leaveStep(tl, "29 parseForwardStep")
	return ret, nil
}

// [30] ForwardAxis ::= ("child" "::") | ("descendant" "::")| ("attribute" "::")| ("self" "::")| ("descendant-or-self" "::")| ("following-sibling" "::")| ("following" "::")| ("namespace" "::")
// [32] ReverseStep ::= (ReverseAxis NodeTest) | AbbrevReverseStep
// [34] AbbrevReverseStep ::= ".."
// [33] ReverseAxis ::= ("parent" "::") | ("ancestor" "::") | ("preceding-sibling" "::") | ("preceding" "::") | ("ancestor-or-self" "::")
// [35] NodeTest ::= KindTest | NameTest
func parseNodeTest(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "35 parseNodeTest")
	var tf testFunc
	var err error
	if str, found := tl.readNexttokIfIsOneOfValueAndType(kindTestStrings, tokQName); found {
		if tf, err = parseKindTest(tl, str); err != nil {
			return nil, err
		}
		if tf != nil {
			return tf, nil
		}
	}
	if tf, err = parseNameTest(tl); err != nil {
		leaveStep(tl, "35 parseNodeTest (err)")
		return nil, err
	}

	leaveStep(tl, "35 parseNodeTest")
	return tf, nil
}

// [36] NameTest ::= QName | Wildcard
func parseNameTest(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "36 parseNameTest")
	var tf testFunc

	if tl.nexttokIsTyp(tokQName) {
		n, err := tl.read()
		if err != nil {
			leaveStep(tl, "36 parseNameTest (err)")
			return nil, err
		}
		var name string
		var ok bool
		if name, ok = n.Value.(string); !ok {
			return nil, err
		}
		if tl.attributeMode {
			tf = returnAttributeNameTest(name)
		} else {
			tf = returnElementNameTest(name)
		}

		leaveStep(tl, "36 parseNameTest")
		return tf, nil
	}
	var err error
	tf, err = parseWildCard(tl)
	if err != nil {
		leaveStep(tl, "36 parseNameTest (err)")
		return nil, err
	}
	leaveStep(tl, "36 parseNameTest")
	return tf, nil
}

// [37] Wildcard ::= "*" | (NCName ":" "*") | ("*" ":" NCName)
func parseWildCard(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "37 parseWildCard")
	var tf testFunc
	var err error
	var strTok *token
	if strTok, err = tl.read(); err != nil {
		leaveStep(tl, "37 parseWildCard (err)")
		return nil, err
	}

	if str, ok := strTok.Value.(string); ok {
		if str == "*" || strings.HasPrefix(str, "*:") || strings.HasSuffix(str, ":*") {
			if tl.attributeMode {
				tf = func(itm Item) bool {
					if _, ok := itm.(*goxml.Attribute); ok {
						return true
					}
					return false
				}
			} else {
				tf = func(itm Item) bool {
					if _, ok := itm.(*goxml.Element); ok {
						return true
					}
					return false
				}
			}

		} else {
			tl.unread()
		}
	} else {
		tl.unread()
	}
	leaveStep(tl, "37 parseWildCard")
	return tf, nil
}

// [38] FilterExpr ::= PrimaryExpr PredicateList
// [39] PredicateList ::= Predicate*
// [40] Predicate ::= "[" Expr "]"
func parseFilterExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "38 parseFilterExpr")

	var ef EvalFunc
	ef, err := parsePrimaryExpr(tl)
	if err != nil {
		leaveStep(tl, "38 parseFilterExpr (err)")
		return nil, err
	}
	predicates := []EvalFunc{}

	for {
		if tl.nexttokIsTyp(tokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				return nil, err
			}
			predicates = append(predicates, predicate)
			err = tl.skipType(tokCloseBracket)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	if len(predicates) == 0 {
		leaveStep(tl, "29 parseForwardStep (#p == 0)")
		return ef, nil
	}
	ff := func(ctx *Context) (Sequence, error) {
		ctx.sequence, err = ef(ctx)
		if err != nil {
			return nil, err
		}
		for _, predicate := range predicates {
			_, err = ctx.Filter(predicate)
			if err != nil {
				return nil, err
			}
		}
		ctx.ctxPositions = nil
		ctx.ctxLengths = nil
		return ctx.sequence, nil
	}
	leaveStep(tl, "38 parseFilterExpr")
	return ff, nil
}

// [41] PrimaryExpr ::= Literal | VarRef | ParenthesizedExpr | ContextItemExpr | FunctionCall
func parsePrimaryExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "41 parsePrimaryExpr")
	var ef EvalFunc

	nexttok, err := tl.read()
	if err != nil {
		leaveStep(tl, "41 parsePrimaryExpr (err) ")
		return nil, err
	}

	// StringLiteral
	if nexttok.Typ == tokString {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{nexttok.Value.(string)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// NumericLiteral
	if nexttok.Typ == tokNumber {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{nexttok.Value.(float64)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// ParenthesizedExpr
	if nexttok.Typ == tokOpenParen {
		ef, err = parseParenthesizedExpr(tl)
		if err != nil {
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// VarRef
	if nexttok.Typ == tokVarname {
		varname := nexttok.Value.(string)
		ef = func(ctx *Context) (Sequence, error) {
			return ctx.vars[varname], nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// Context item
	if nexttok.Typ == tokOperator && nexttok.Value.(string) == "." {
		ef = func(ctx *Context) (Sequence, error) {
			return ctx.sequence, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// FunctionCall
	if tl.nexttokIsTyp(tokOpenParen) {
		tl.unread() // function name
		fname := nexttok.String()
		if fname == "text" || fname == "element" || fname == "attribute" || fname == "node" {
			return nil, nil
		}
		ef, err := parseFunctionCall(tl)
		if err != nil {
			leaveStep(tl, "41 parsePrimaryExpr (err)")
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr (fc)")
		return ef, nil
	}
	tl.unread()
	leaveStep(tl, "41 parsePrimaryExpr")
	return nil, nil
}

// [46] ParenthesizedExpr ::= "(" Expr? ")"
func parseParenthesizedExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "46 parseParenthesizedExpr")
	var exp, ef EvalFunc
	var err error
	exp, err = parseExpr(tl)
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(tokCloseParen); err != nil {
		return nil, err
	}

	ef = func(ctx *Context) (Sequence, error) {
		seq, err := exp(ctx)
		if err != nil {
			return nil, err
		}

		return seq, nil
	}

	leaveStep(tl, "46 parseParenthesizedExpr")
	return ef, nil
}

// [48] FunctionCall ::= QName "(" (ExprSingle ("," ExprSingle)*)? ")"
func parseFunctionCall(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "48 parseFunctionCall")
	var ef EvalFunc

	functionNameToken, err := tl.read()
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(tokOpenParen); err != nil {
		return nil, err
	}
	fn := functionNameToken.Value.(string)
	if tl.nexttokIsTyp(tokCloseParen) {
		tl.read()
		ef = func(ctx *Context) (Sequence, error) {
			return callFunction(fn, []Sequence{}, ctx)
		}
		leaveStep(tl, "48 parseFunctionCall (a)")
		return ef, nil
	}

	var efs []EvalFunc

	for {
		es, err := parseExprSingle(tl)
		if err != nil {
			return nil, err
		}
		efs = append(efs, es)
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read()
	}

	if err = tl.skipType(tokCloseParen); err != nil {
		leaveStep(tl, "48 parseFunctionCall (err)")
		return nil, fmt.Errorf("close paren expected")
	}

	// get expr single *
	ef = func(ctx *Context) (Sequence, error) {
		var arguments []Sequence
		saveContext := ctx.GetContextSequence()
		for _, es := range efs {
			seq, err := es(ctx)
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, seq)
			ctx.SetContextSequence(saveContext)
		}

		return callFunction(fn, arguments, ctx)
	}

	leaveStep(tl, "48 parseFunctionCall")
	return ef, nil
}

// orig: [50] SequenceType ::= ("empty-sequence" "(" ")")| (ItemType OccurrenceIndicator?)
// since empty-sequence() is implemented in 16 and 17, we can skip this here:
// [50] SequenceType ::= ItemType OccurrenceIndicator?
func parseSequenceType(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "50 parseSequenceType")
	var tf testFunc
	var err error

	tf, err = parseItemType(tl)
	if err != nil {
		leaveStep(tl, "50 parseSequenceType (err)")
		return nil, err
	}

	leaveStep(tl, "50 parseSequenceType")
	return tf, nil
}

// [52] ItemType ::= KindTest | ("item" "(" ")") | AtomicType
func parseItemType(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "52 parseItemType")
	var tf testFunc
	var err error

	if str, found := tl.readNexttokIfIsOneOfValueAndType(kindTestStrings, tokQName); found {
		if tf, err = parseKindTest(tl, str); err != nil {
			return nil, err
		}
		if tf != nil {
			return tf, nil
		}
	}

	leaveStep(tl, "52 parseItemType")
	return tf, nil
}

// [51] OccurrenceIndicator ::= "?" | "*" | "+"
// [53] AtomicType ::= QName
// [54] KindTest ::= DocumentTest|
// 					 ElementTest |
// 					 AttributeTest |
// 					 SchemaElementTest|
//			 		 SchemaAttributeTest|
//			 		 PITest|
//			 		 CommentTest|
//			 		 TextTest|
//			 		 AnyKindTest
// [67] ElementDeclaration ::= ElementName
// [56] DocumentTest ::= "document-node" "(" (ElementTest | SchemaElementTest)? ")"
// [60] AttributeTest ::= "attribute" "(" (AttribNameOrWildcard ("," TypeName)?)? ")"
// [61] AttribNameOrWildcard ::= AttributeName | "*"
// [68] AttributeName ::= QName
// [66] SchemaElementTest ::= "schema-element" "(" ElementDeclaration ")"
// [62] SchemaAttributeTest ::= "schema-attribute" "(" AttributeDeclaration ")"
// [63] AttributeDeclaration ::= AttributeName
// [59] PITest ::= "processing-instruction" "(" (NCName | StringLiteral)? ")"
// [64] ElementTest ::= "element" "(" (ElementNameOrWildcard ("," TypeName "?"?)?)? ")"
// [65] ElementNameOrWildcard ::= ElementName | "*"
// [69] ElementName ::= QName
// [70] TypeName ::= QName
// [58] CommentTest ::= "comment" "(" ")"
// [57] TextTest ::= "text" "(" ")"
// [55] AnyKindTest ::= "node" "(" ")"

var kindTestStrings = []string{"element", "node", "text", "attribute", "document-node", "schema-element", "schema-attribute", "processing-instruction", "comment"}

func parseKindTest(tl *Tokenlist, name string) (testFunc, error) {
	enterStep(tl, "54 parseKindTest")
	var tf testFunc
	var err error
	if err = tl.skipType(tokOpenParen); err != nil {
		return nil, err
	}
	switch name {
	case "element":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		leaveStep(tl, "35 parseNodeTest")
		return isElement, nil
	case "node":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		tf := func(itm Item) bool {
			return true
		}
		leaveStep(tl, "35 parseNodeTest")
		return tf, nil
	case "text":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		tf := func(itm Item) bool {
			if _, ok := itm.(goxml.CharData); ok {
				return true
			}
			return false
		}
		leaveStep(tl, "35 parseNodeTest")
		return tf, nil
	case "attribute":
		nexttok, err := tl.peek()
		if err != nil {
			return nil, err
		}
		if nexttok.Value == ')' {
			tl.read()
			leaveStep(tl, "35 parseNodeTest")
			return isAttribute, nil
		} else if nexttok.Value == "*" && nexttok.Typ == tokOperator {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return isAttribute, nil
		} else if nexttok.Typ == tokQName {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return returnAttributeNameTest(nexttok.String()), nil
		}
	default:
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("not implemented yet %s", name)
	}

	leaveStep(tl, "54 parseKindTest")
	return tf, nil
}

// ParseXPath takes a previously created token list and returns a function that
// can be used to evaluate the XPath expression in different contexts.
func ParseXPath(tl *Tokenlist) (EvalFunc, error) {
	ef, err := parseExpr(tl)
	if err != nil {
		return nil, err
	}
	return ef, nil
}

// Parser contains all necessary references to the parser
type Parser struct {
	Ctx *Context
}

// XMLDocument returns the underlying XML document
func (xp *Parser) XMLDocument() *goxml.XMLDocument {
	return xp.Ctx.xmldoc
}

// SetVariable is used to set a variable name.
func (xp *Parser) SetVariable(name string, value Sequence) {
	xp.Ctx.vars[name] = value
}

// Evaluate reads an XPath expression and evaluates it in the given context.
func (xp *Parser) Evaluate(xpath string) (Sequence, error) {
	tl, err := stringToTokenlist(xpath)
	if err != nil {
		return nil, err
	}
	evaler, err := ParseXPath(tl)
	if err != nil {
		return nil, err
	}

	return evaler(xp.Ctx)
}

// NewParser returns a context to be filled
func NewParser(r io.Reader) (*Parser, error) {
	xp := &Parser{}

	doc, err := goxml.Parse(r)
	if err != nil {
		return nil, err
	}

	xp.Ctx = NewContext(doc)
	return xp, nil
}
