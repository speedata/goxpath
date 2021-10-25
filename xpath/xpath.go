package xpath

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/speedata/goxml"
)

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

// Context is needed for variables and XML navigation.
type Context struct {
	vars         map[string]Sequence
	context      Sequence
	ctxPositions []int
	ctxLengths   []int
	pos          int
	size         int
	xmldoc       *goxml.XMLDocument
}

// NewContext returns a context from the xml document
func NewContext(doc *goxml.XMLDocument) *Context {
	ctx := &Context{
		xmldoc: doc,
		vars:   make(map[string]Sequence),
	}

	return ctx
}

// Document moves the node navigator to the document and retuns it
func (ctx *Context) Document() goxml.XMLNode {
	ctx.context = Sequence{ctx.xmldoc}
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
	ctx.context = Sequence{cur}
	ctx.ctxPositions = nil
	ctx.ctxLengths = nil
	return ctx.context, err
}

type testfuncChildren func(*goxml.Element) bool
type testfuncAttributes func(*goxml.Attribute) bool

// Child returns all children of the current node that satisfy the testfunc
func (ctx *Context) Child(tf testfuncChildren) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	ctx.ctxLengths = []int{}
	for _, n := range ctx.context {
		if node, ok := n.(goxml.XMLNode); ok {
			pos := 0
			l := 0
			for _, cld := range node.Children() {
				if celt, ok := cld.(*goxml.Element); ok {
					if tf(celt) {
						pos++
						l++
						ctx.ctxPositions = append(ctx.ctxPositions, pos)
						seq = append(seq, celt)
					}
				}
			}
			for i := 0; i < l; i++ {
				ctx.ctxLengths = append(ctx.ctxLengths, l)
			}
		}
	}
	ctx.context = seq
	return seq, nil
}

// Attributes returns all attributes of the current node that satisfy the testfunc
func (ctx *Context) Attributes(tf testfuncAttributes) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	for _, n := range ctx.context {
		if node, ok := n.(goxml.XMLNode); ok {
			if elt, ok := node.(*goxml.Element); ok {
				for _, attr := range elt.Attributes() {
					if tf(attr) {
						ctx.ctxPositions = append(ctx.ctxPositions, 1)
						seq = append(seq, attr)
					}
				}
			}
		}
	}
	ctx.context = seq
	return seq, nil
}

// Filter applies prediates to the context
func (ctx *Context) Filter(filter evalFunc) (Sequence, error) {
	var result Sequence
	var predicateIsNum bool
	var predicateNum int

	var lengths []int
	var positions []int

	if ctx.ctxPositions != nil {
		positions = ctx.ctxPositions
		lengths = ctx.ctxLengths
	} else {
		positions = make([]int, len(ctx.context))
		lengths = make([]int, len(ctx.context))
		for i := 0; i < len(ctx.context); i++ {
			positions[i] = i + 1
			lengths[i] = 1
		}
	}

	copyContext := ctx.context
	ctx.size = lengths[0]

	predicate, err := filter(ctx)
	if err != nil {
		return nil, err
	}
	if len(predicate) == 1 {
		if p0, ok := predicate[0].(float64); ok {
			predicateIsNum = true
			predicateNum = int(p0)
		} else if p0, ok := predicate[0].(int); ok {
			predicateIsNum = true
			predicateNum = p0
		}
	}
	if predicateIsNum {
		var seq Sequence
		for i, itm := range ctx.context {
			pos := positions[i]
			if predicateNum == pos {
				seq = append(seq, itm)
			}
		}
		return seq, nil
	}

	for i, itm := range copyContext {
		ctx.context = Sequence{itm}
		ctx.pos = positions[i]
		ctx.size = lengths[i]
		predicate, err := filter(ctx)
		if err != nil {
			return nil, err
		}
		evalItem, err := booleanValue(predicate)
		if err != nil {
			return nil, err
		}
		if evalItem {
			result = append(result, itm)
		}
	}

	if len(result) == 0 {
		result = Sequence{}
	}
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

// type compareFunc func(interface{}, interface{}) (bool, error)

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

func (s Sequence) stringvalue() string {
	var sb strings.Builder
	for _, itm := range s {
		switch t := itm.(type) {
		case float64:
			fmt.Fprintf(&sb, "%f", t)
		case *goxml.Attribute:
			fmt.Fprintf(&sb, t.Value)
		default:
			fmt.Fprintf(&sb, "%s", t)
		}
	}
	return sb.String()
}

type evalFunc func(*Context) (Sequence, error)

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
	if intRight, ok = b.(int); ok {
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

	return false, fmt.Errorf("FORG0001")
}

func doCompare(op string, lhs evalFunc, rhs evalFunc) (evalFunc, error) {
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

func numberValue(s Sequence) (float64, error) {
	if len(s) == 0 {
		return math.NaN(), nil
	}
	if len(s) > 1 {
		return math.NaN(), fmt.Errorf("Required cardinality of first argument of fn:number() is zero or one; supplied value has cardinality more than one")
	}
	if num, ok := s[0].(int); ok {
		return float64(num), nil
	}

	if flt, ok := s[0].(float64); ok {
		return flt, nil
	}
	return math.NaN(), nil
}

func booleanValue(s Sequence) (bool, error) {
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
		} else {
			fmt.Printf("itm %#v\n", itm)
		}
	}
	return false, fmt.Errorf("FORG0006 Invalid argument type")
}

//  [2] Expr ::= ExprSingle ("," ExprSingle)*
func parseExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "2 parseExpr")
	var efs []evalFunc
	for {
		ef, err := parseExprSingle(tl)
		if err != nil {
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsTyp(TokComma) {
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
func parseExprSingle(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "3 parseExprSingle")
	var ef evalFunc
	var err error
	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"for", "some", "if"}); ok {
		switch op {
		case "for":
			ef, err = parseForExpr(tl)
		case "some":
			return nil, fmt.Errorf("not implemented yet: some")
			// ef = parseQuantifiedExpr(tl)
		case "if":
			leaveStep(tl, "3 parseExprSingle")
			ef, err = parseIfExpr(tl)
		}
		return ef, err
	}

	ef, err = parseOrExpr(tl)
	if err != nil {
		leaveStep(tl, "3 parseExprSingle")
		return nil, err
	}
	leaveStep(tl, "3 parseExprSingle")
	return ef, nil
}

// [4] ForExpr ::= SimpleForClause "return" ExprSingle
// [5] SimpleForClause ::= "for" "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)*
func parseForExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "4 parseForExpr")
	var simpleForClauseF evalFunc
	var err error
	var varname string
	// simple for clause:
	vartoken, err := tl.read()
	if err != nil {
		return nil, err
	}
	if vn, ok := vartoken.Value.(string); ok {
		varname = vn
	} else {
		return nil, fmt.Errorf("variable name not a string")
	}

	if err = tl.skipNCName("in"); err != nil {
		return nil, err
	}
	if simpleForClauseF, err = parseExprSingle(tl); err != nil {
		return nil, err
	}

	err = tl.skipNCName("return")
	if err != nil {
		return nil, err
	}

	evalseq, err := parseExprSingle(tl)
	ret := func(ctx *Context) (Sequence, error) {
		var SimpleForClauseSeq Sequence
		var retSeq Sequence

		SimpleForClauseSeq, err = simpleForClauseF(ctx)
		if err != nil {
			return nil, err
		}
		for _, itm := range SimpleForClauseSeq {
			// TODO: save variable value and restore afterwards
			ctx.vars[varname] = Sequence{itm}
			seq, err := evalseq(ctx)
			if err != nil {
				return nil, err
			}
			retSeq = append(retSeq, seq...)
		}
		return retSeq, nil
	}
	leaveStep(tl, "4 parseForExpr")
	return ret, nil
}

// [6] QuantifiedExpr ::= ("some" | "every") "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)* "satisfies" ExprSingle

// [7] IfExpr ::= "if" "(" Expr ")" "then" ExprSingle "else" ExprSingle
func parseIfExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "7 parseIfExpr")
	var nexttok *token
	var err error
	var boolEval, thenpart, elsepart evalFunc

	if nexttok, err = tl.read(); err != nil {
		return nil, err
	}
	if nexttok.Typ != TokOpenParen {
		return nil, fmt.Errorf("open parenthesis expected, found %v", nexttok.Value)
	}
	if boolEval, err = parseExpr(tl); err != nil {
		return nil, err
	}
	if err = tl.skipType(TokCloseParen); err != nil {
		return nil, err
	}
	if err = tl.skipNCName("then"); err != nil {
		return nil, err
	}
	if thenpart, err = parseExprSingle(tl); err != nil {
		return nil, err
	}
	if err = tl.skipNCName("else"); err != nil {
		return nil, err
	}
	if elsepart, err = parseExprSingle(tl); err != nil {
		return nil, err
	}

	f := func(ctx *Context) (Sequence, error) {
		res, err := boolEval(ctx)
		if err != nil {
			return nil, err
		}
		bv, err := booleanValue(res)
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
func parseOrExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "8 parseOrExpr")
	var ef evalFunc
	var efs []evalFunc
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
		leaveStep(tl, "8 parseOrExpr")
		return efs[0], nil
	}
	ef = func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := booleanValue(s)
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
func parseAndExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "9 parseAndExpr")

	var ef evalFunc
	var efs []evalFunc
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
		leaveStep(tl, "9 parseAndExpr")
		return efs[0], nil
	}

	ef = func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := booleanValue(s)
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
func parseComparisonExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "10 parseComparisonExpr")
	var lhs, rhs evalFunc
	var err error
	if lhs, err = parseRangeExpr(tl); err != nil {
		leaveStep(tl, "10 parseComparisonExpr")
		return nil, err
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"=", "<", ">", "<=", ">=", "!=", "eq", "ne", "lt", "le", "gt", "ge", "is", "<<", ">>"}); ok {
		if rhs, err = parseRangeExpr(tl); err != nil {
			return nil, err
		}
		leaveStep(tl, "10 parseComparisonExpr")
		return doCompare(op, lhs, rhs)
	}

	leaveStep(tl, "10 parseComparisonExpr")
	return lhs, nil
}

// [11] RangeExpr  ::=  AdditiveExpr ( "to" AdditiveExpr )?
func parseRangeExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "11 parseRangeExpr")
	var ef evalFunc
	var efs []evalFunc
	var err error
	for {
		ef, err = parseAdditiveExpr(tl)
		if err != nil {
			leaveStep(tl, "11 parseRangeExpr")
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
		leaveStep(tl, "11 parseRangeExpr")
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
		lhsNum, err := numberValue(lhs)
		if err != nil {
			return nil, err
		}
		rhsNum, err := numberValue(rhs)
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
func parseAdditiveExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "12 parseAdditiveExpr")
	var efs []evalFunc
	var operator []string
	var ef evalFunc
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
		sum, err := numberValue(s)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(efs); i++ {
			s, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			flt, err := numberValue(s)
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
func parseMultiplicativeExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "13 parseMultiplicativeExpr")

	var efs []evalFunc
	var operator []string
	var ef evalFunc
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
		sum, err := numberValue(s)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(efs); i++ {
			s, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			flt, err := numberValue(s)
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
func parseUnionExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "14 parseUnionExpr")
	var ef evalFunc

	ef, err := parseIntersectExceptExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "14 parseUnionExpr")
	return ef, nil
}

// [15] IntersectExceptExpr  ::= InstanceofExpr ( ("intersect" | "except") InstanceofExpr )*
func parseIntersectExceptExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "15 parseIntersectExceptExpr")
	var ef evalFunc
	ef, err := parseInstanceofExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "15 parseIntersectExceptExpr")
	return ef, nil
}

// [16] InstanceofExpr ::= TreatExpr ( "instance" "of" SequenceType )?
func parseInstanceofExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "16 parseInstanceofExpr")
	var ef evalFunc
	ef, err := parseTreatExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "16 parseInstanceofExpr")
	return ef, nil
}

// [17] TreatExpr ::= CastableExpr ( "treat" "as" SequenceType )?
func parseTreatExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "17 parseTreatExpr")
	var ef evalFunc
	ef, err := parseCastableExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "17 parseTreatExpr")
	return ef, nil
}

// [18] CastableExpr ::= CastExpr ( "castable" "as" SingleType )?
func parseCastableExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "18 parseCastableExpr")
	var ef evalFunc
	ef, err := parseCastExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "18 parseCastableExpr")
	return ef, nil
}

// [19] CastExpr ::= UnaryExpr ( "cast" "as" SingleType )?
func parseCastExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "19 parseCastExpr")
	var ef evalFunc
	ef, err := parseUnaryExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "19 parseCastExpr")
	return ef, nil
}

// [20] UnaryExpr ::= ("-" | "+")* ValueExpr
func parseUnaryExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "20 parseUnaryExpr")
	var ef evalFunc
	mult := 1
	for {
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"+", "-"}); ok {
			if op == "-" {
				mult *= -1
			}
		} else {
			break
		}
	}
	pv, err := parseValueExpr(tl)
	if err != nil {
		return nil, err
	}

	ef = func(ctx *Context) (Sequence, error) {
		if mult == -1 {
			seq, err := pv(ctx)
			if err != nil {
				return nil, err
			}
			flt, err := numberValue(seq)
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
func parseValueExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "21 parseValueExpr")
	var ef evalFunc
	ef, err := parsePathExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "21 parseValueExpr")
	return ef, nil
}

// [25] PathExpr ::= ("/" RelativePathExpr?) | ("//" RelativePathExpr) | RelativePathExpr
func parsePathExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "25 parsePathExpr")
	var rpe evalFunc
	var op string
	var hasOP bool
	op, hasOP = tl.readNexttokIfIsOneOfValue([]string{"/", "//"})

	rpe, err := parseRelativePathExpr(tl)
	if err != nil {
		return nil, err
	}

	if hasOP {
		switch op {
		case "/":
			fn := func(ctx *Context) (Sequence, error) {
				ctx.Document()
				seq, err := rpe(ctx)
				if err != nil {
					return nil, err
				}
				return seq, nil
			}
			return fn, nil
		case "//":
			panic("nyi")
		}
	}

	leaveStep(tl, "25 parsePathExpr")
	return rpe, nil
}

// [26] RelativePathExpr ::= StepExpr (("/" | "//") StepExpr)*
func parseRelativePathExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "26 parseRelativePathExpr")
	var ef evalFunc
	var efs []evalFunc
	var ops []string
	var err error

	for {
		ef, err := parseStepExpr(tl)
		if err != nil {
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
		leaveStep(tl, "26 parseRelativePathExpr")
		return efs[0], nil // just a simple StepExpr
	}

	ef = func(ctx *Context) (Sequence, error) {
		var seq Sequence
		for i := 0; i < len(efs); i++ {

			if seq, err = efs[i](ctx); err != nil {
				return nil, err
			}
			// fmt.Printf("26 seq %#v\n", seq)
			// switch ops[i] {
			// case "/":
			// 	fmt.Println("/")
			// }
		}

		return seq, nil
	}

	leaveStep(tl, "26 parseRelativePathExpr")
	return ef, nil
}

// [27] StepExpr := FilterExpr | AxisStep
func parseStepExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "27 parseStepExpr")
	var ef evalFunc
	ef, err := parseFilterExpr(tl)
	if err != nil {
		return nil, err
	}
	if ef == nil {
		ef, err = parseAxisStepExpr(tl)
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
func parseAxisStepExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "28 parseAxisStepExpr")
	var ef evalFunc
	var err error
	if ef, err = parseForwardStepExpr(tl); err != nil {
		leaveStep(tl, "28 parseAxisStepExpr (err)")
		return nil, err
	}
	for {
		if tl.nexttokIsTyp(TokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				return nil, err
			}
			err = tl.skipType(TokCloseBracket)
			if err != nil {
				return nil, err
			}
			ff := func(ctx *Context) (Sequence, error) {
				_, err = ef(ctx)
				if err != nil {
					return nil, err
				}
				return ctx.Filter(predicate)
			}
			leaveStep(tl, "28 parseAxisStepExpr (a)")
			return ff, nil
		}
		break
	}

	leaveStep(tl, "28 parseAxisStepExpr (b)")
	return ef, nil
}

// [29] ForwardStep ::= (ForwardAxis NodeTest) | AbbrevForwardStep
// [31] AbbrevForwardStep ::= "@"? NodeTest
func parseForwardStepExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, " 29 parseForwardStepExpr")
	var ef evalFunc
	var err error

	if tl.nexttokIsValue("@") {
		tl.read()
		tl.attributeMode = true
	} else {
		tl.attributeMode = false
	}

	if ef, err = parseNodeTest(tl); err != nil {
		return nil, err
	}

	leaveStep(tl, "29 parseForwardStepExpr")
	return ef, nil
}

// [30] ForwardAxis ::= ("child" "::") | ("descendant" "::")| ("attribute" "::")| ("self" "::")| ("descendant-or-self" "::")| ("following-sibling" "::")| ("following" "::")| ("namespace" "::")
// [32] ReverseStep ::= (ReverseAxis NodeTest) | AbbrevReverseStep
// [34] AbbrevReverseStep ::= ".."
// [33] ReverseAxis ::= ("parent" "::") | ("ancestor" "::") | ("preceding-sibling" "::") | ("preceding" "::") | ("ancestor-or-self" "::")
// [35] NodeTest ::= KindTest | NameTest
func parseNodeTest(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "35 parseNodeTestExpr")
	var ef evalFunc
	var err error
	if ef, err = parseNameTest(tl); err != nil {
		leaveStep(tl, "35 parseNodeTestExpr (err)")
		return nil, err
	}

	leaveStep(tl, "35 parseNodeTestExpr")
	return ef, nil
}

// [36] NameTest ::= QName | Wildcard
func parseNameTest(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "36 parseNameTest")
	var ef evalFunc
	var err error
	if tl.nexttokIsTyp(TokQName) {
		n, err := tl.read()
		if err != nil {
			return nil, err
		}
		if tl.attributeMode {
			ef = func(ctx *Context) (Sequence, error) {
				ctx.Attributes(returnIsNameTFAttr(n.Value.(string)))
				return ctx.context, nil
			}
		} else {
			ef = func(ctx *Context) (Sequence, error) {
				ctx.Child(returnIsNameTF(n.Value.(string)))
				return ctx.context, nil
			}
		}

		leaveStep(tl, "36 parseNameTest")
		return ef, nil
	}
	// leaveStep(tl, "36 parseNameTest (err)")
	// return nil, fmt.Errorf("nametest failed")

	ef, err = parseWildCard(tl)
	if err != nil {
		return nil, err
	}
	leaveStep(tl, "36 parseNameTest")
	return ef, nil
}

// [37] Wildcard ::= "*" | (NCName ":" "*") | ("*" ":" NCName)
func parseWildCard(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "37 parseWildCard")
	var ef evalFunc
	var err error
	var strTok *token
	if strTok, err = tl.read(); err != nil {
		leaveStep(tl, "37 parseWildCard (err)")
		return nil, err
	}

	if str, ok := strTok.Value.(string); ok {
		if str == "*" || strings.HasPrefix(str, "*:") || strings.HasSuffix(str, ":*") {
			if tl.attributeMode {
				ef = func(ctx *Context) (Sequence, error) {
					ctx.Attributes(returnIsNameTFAttr(str))
					return ctx.context, nil
				}

			} else {
				ef = func(ctx *Context) (Sequence, error) {
					ctx.Child(returnIsNameTF(str))
					return ctx.context, nil
				}

			}
		}
	} else {
		tl.unread()
	}
	leaveStep(tl, "37 parseWildCard")
	return ef, nil
}

// [38] FilterExpr ::= PrimaryExpr PredicateList
// [39] PredicateList ::= Predicate*
func parseFilterExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "38 parseFilterExpr")
	var ef evalFunc
	ef, err := parsePrimaryExpr(tl)
	if err != nil {
		return nil, err
	}
	for {
		if tl.nexttokIsTyp(TokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				return nil, err
			}
			err = tl.skipType(TokCloseBracket)
			if err != nil {
				return nil, err
			}

			ff := func(ctx *Context) (Sequence, error) {
				ctx.context, err = ef(ctx)
				if err != nil {
					return nil, err
				}
				ctx.ctxPositions = nil
				ctx.ctxLengths = nil
				return ctx.Filter(predicate)
			}
			leaveStep(tl, "38 parseFilterExpr")
			return ff, nil
		}
		break
	}
	leaveStep(tl, "38 parseFilterExpr")
	return ef, nil
}

// [40] Predicate ::= "[" Expr "]"
// [41] PrimaryExpr ::= Literal | VarRef | ParenthesizedExpr | ContextItemExpr | FunctionCall
func parsePrimaryExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "41 parsePrimaryExpr")
	var ef evalFunc

	nexttok, err := tl.read()
	if err != nil {
		return nil, err
	}

	// StringLiteral
	if nexttok.Typ == TokString {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{nexttok.Value.(string)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// NumericLiteral
	if nexttok.Typ == TokNumber {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{nexttok.Value.(float64)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// ParenthesizedExpr
	if nexttok.Typ == TokOpenParen {
		ef, err = parseParenthesizedExpr(tl)
		if err != nil {
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// VarRef
	if nexttok.Typ == TokVarname {
		ef = func(ctx *Context) (Sequence, error) {
			return ctx.vars[nexttok.Value.(string)], nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// FunctionCall
	if tl.nexttokIsTyp(TokOpenParen) {
		tl.unread() // function name
		ef, err := parseFunctionCall(tl)
		if err != nil {
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}
	tl.unread()
	leaveStep(tl, "41 parsePrimaryExpr")
	return nil, nil
}

// [46] ParenthesizedExpr ::= "(" Expr? ")"
func parseParenthesizedExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "46 parseParenthesizedExpr")
	var exp, ef evalFunc
	var err error
	exp, err = parseExpr(tl)
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(TokCloseParen); err != nil {
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

//  [48] FunctionCall ::= QName "(" (ExprSingle ("," ExprSingle)*)? ")"
func parseFunctionCall(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "48 parseFunctionCall")
	var ef evalFunc

	functionNameToken, err := tl.read()
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(TokOpenParen); err != nil {
		return nil, err
	}
	functionName := functionNameToken.Value.(string)
	if !hasFunction(functionName) {
		return nil, fmt.Errorf("function %q not defined", functionName)
	}

	if tl.nexttokIsTyp(TokCloseParen) {
		tl.read()
		ef = func(ctx *Context) (Sequence, error) {
			return callFunction(functionName, []Sequence{}, ctx)
		}
		leaveStep(tl, "48 parseFunctionCall")

		return ef, nil
	}

	var efs []evalFunc

	for {
		es, err := parseExprSingle(tl)
		if err != nil {
			return nil, err
		}
		efs = append(efs, es)
		if !tl.nexttokIsTyp(TokComma) {
			break
		}
		tl.read()
	}

	if err = tl.skipType(TokCloseParen); err != nil {
		leaveStep(tl, "48 parseFunctionCall (err)")
		return nil, fmt.Errorf("close paren expected")
	}

	// get expr single *
	ef = func(ctx *Context) (Sequence, error) {
		var arguments []Sequence
		for _, es := range efs {
			seq, err := es(ctx)
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, seq)
		}
		return callFunction(functionName, arguments, ctx)
	}

	leaveStep(tl, "48 parseFunctionCall")
	return ef, nil
}

// [54] KindTest ::= DocumentTest| ElementTest| AttributeTest| SchemaElementTest| SchemaAttributeTest| PITest| CommentTest| TextTest| AnyKindTest

func parseXPath(tl *tokenlist) (evalFunc, error) {
	ef, err := parseExpr(tl)
	if err != nil {
		return nil, err
	}
	return ef, nil
}

// Parser contains all necessary references to the parser
type Parser struct {
	ctx *Context
}

// SetVariable is used to set a variable name.
func (xp *Parser) SetVariable(name string, value Sequence) {
	xp.ctx.vars[name] = value
}

// Evaluate evaluates an xpath expression
func (xp *Parser) Evaluate(xpath string) (Sequence, error) {
	tl, err := stringToTokenlist(xpath)
	if err != nil {
		return nil, err
	}
	evaler, err := parseXPath(tl)
	if err != nil {
		return nil, err
	}

	return evaler(xp.ctx)
}

// NewParser returns a context to be filled
func NewParser(r io.Reader) (*Parser, error) {
	xp := &Parser{}

	doc, err := goxml.Parse(r)
	if err != nil {
		return nil, err
	}

	xp.ctx = NewContext(doc)
	return xp, nil
}
