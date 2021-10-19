package xpath

import (
	"fmt"
	"io"
	"math"
	"strings"
)

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

type context struct {
	vars    map[string]sequence
	tmpvars map[string]sequence
	context sequence
	current sequence
}

func (ctx *context) Filter(filter evalFunc) (sequence, error) {
	var result sequence
	var predicateIsNum bool
	var predicateNum int

	predicate, err := filter(ctx)
	if err != nil {
		return nil, err
	}
	if len(predicate) == 1 {
		if p0, ok := predicate[0].(float64); ok {
			predicateIsNum = true
			predicateNum = int(p0)
		}
	}
	if predicateIsNum {
		if predicateNum > len(ctx.context) {
			return sequence{}, nil
		}
		return sequence{ctx.context[predicateNum-1]}, nil
	}
	copyContext := ctx.context
	for _, itm := range copyContext {
		ctx.context = sequence{itm}
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
	return result, nil
}

type item interface{}

// type compareFunc func(interface{}, interface{}) (bool, error)

type sequence []item

func (s sequence) String() string {
	var sb strings.Builder
	sb.WriteString(`( `)
	for _, itm := range s {
		fmt.Fprintf(&sb, "%v ", itm)
	}
	sb.WriteString(`)`)
	return sb.String()
}

func (s sequence) stringvalue() string {
	var sb strings.Builder
	for _, itm := range s {
		fmt.Fprintf(&sb, "%s", itm)
	}
	return sb.String()
}

type evalFunc func(*context) (sequence, error)

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

func compareFunc(op string, a, b interface{}) (bool, error) {
	if left, ok := a.(float64); ok {
		if right, ok := b.(float64); ok {
			return doCompareFloat(op, left, right)
		}
	}
	if left, ok := a.(string); ok {
		if right, ok := b.(string); ok {
			return doCompareString(op, left, right)
		}
	}

	return false, fmt.Errorf("FORG0001")
}

func doCompare(op string, lhs evalFunc, rhs evalFunc) (evalFunc, error) {
	f := func(ctx *context) (sequence, error) {
		left, err := lhs(ctx)
		if err != nil {
			return nil, err
		}
		right, err := rhs(ctx)
		if err != nil {
			return nil, err
		}
		var isCompare bool
		for _, leftitem := range left {
			for _, rightitem := range right {
				ok, err := compareFunc(op, leftitem, rightitem)
				if err != nil {
					return nil, err
				}
				if ok {
					isCompare = true
					break
				}
			}
		}
		return sequence{isCompare}, nil
	}
	return f, nil
}

func numberValue(s sequence) (float64, error) {
	if len(s) == 0 {
		return math.NaN(), nil
	}
	if len(s) > 1 {
		return math.NaN(), fmt.Errorf("Required cardinality of first argument of fn:number() is zero or one; supplied value has cardinality more than one")
	}

	if flt, ok := s[0].(float64); ok {
		return flt, nil
	}
	return math.NaN(), nil
}

func booleanValue(s sequence) (bool, error) {
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
		efs = append(efs, ef)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
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

	f := func(ctx *context) (sequence, error) {
		var ret sequence
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
	leaveStep(tl, "3 parseExprSingle")
	return ef, nil
}

// [4] ForExpr ::= SimpleForClause "return" ExprSingle
func parseForExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "4 parseForExpr")
	var ef evalFunc
	var err error
	if ef, err = parseSimpleForClause(tl); err != nil {
		return nil, err
	}
	err = tl.skipNCName("return")
	if err != nil {
		return nil, err
	}
	evalseq, err := parseExprSingle(tl)
	ret := func(ctx *context) (sequence, error) {
		_, err = ef(ctx)
		if err != nil {
			return nil, err
		}
		seq, err := evalseq(ctx)
		if err != nil {
			return nil, err
		}
		return seq, nil
	}
	leaveStep(tl, "4 parseForExpr")
	return ret, nil
}

// [5] SimpleForClause ::= "for" "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)*
func parseSimpleForClause(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "5 parseSimpleForClause")
	var ef evalFunc
	vartoken, err := tl.read()
	if err != nil {
		return nil, err
	}
	fmt.Printf("var %#v\n", vartoken)
	if err = tl.skipNCName("in"); err != nil {
		return nil, err
	}
	if ef, err = parseExprSingle(tl); err != nil {
		return nil, err
	}
	ret := func(ctx *context) (sequence, error) {
		seq, err := ef(ctx)
		if err != nil {
			return nil, err
		}
		ctx.tmpvars[vartoken.Value.(string)] = seq

		return nil, nil
	}

	leaveStep(tl, "5 parseSimpleForClause")
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

	f := func(ctx *context) (sequence, error) {
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
	ef = func(ctx *context) (sequence, error) {
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
				return sequence{true}, nil
			}

		}
		return sequence{false}, nil
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

	ef = func(ctx *context) (sequence, error) {
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
				return sequence{false}, nil
			}

		}
		return sequence{true}, nil
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
		return nil, err
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"=", "<", ">", "<=", ">=", "!="}); ok {
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
	retf := func(ctx *context) (sequence, error) {
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
		var seq sequence
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
	ef = func(ctx *context) (sequence, error) {
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
		return sequence{sum}, nil
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

	ef = func(ctx *context) (sequence, error) {
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
			case "idiv", "div":
				sum /= flt
			case "mod":
				sum = math.Mod(sum, flt)
			}
		}
		return sequence{sum}, nil
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

	ef = func(ctx *context) (sequence, error) {
		if mult == -1 {
			seq, err := pv(ctx)
			if err != nil {
				return nil, err
			}
			flt, err := numberValue(seq)
			if err != nil {
				return nil, err
			}
			return sequence{flt * -1}, nil
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
	var ef evalFunc
	ef, err := parseRelativePathExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "25 parsePathExpr")
	return ef, nil
}

// [26] RelativePathExpr ::= StepExpr (("/" | "//") StepExpr)*
func parseRelativePathExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "26 parseRelativePathExpr")
	var ef evalFunc
	ef, err := parseStepExpr(tl)
	if err != nil {
		return nil, err
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

	leaveStep(tl, "27 parseStepExpr")
	return ef, nil
}

// [28] AxisStep ::= (ReverseStep | ForwardStep) PredicateList
// [29] ForwardStep ::= (ForwardAxis NodeTest) | AbbrevForwardStep
// [30] ForwardAxis ::= ("child" "::") | ("descendant" "::")| ("attribute" "::")| ("self" "::")| ("descendant-or-self" "::")| ("following-sibling" "::")| ("following" "::")| ("namespace" "::")
// [32] ReverseStep ::= (ReverseAxis NodeTest) | AbbrevReverseStep
// [34] AbbrevReverseStep ::= ".."
// [33] ReverseAxis ::= ("parent" "::") | ("ancestor" "::") | ("preceding-sibling" "::") | ("preceding" "::") | ("ancestor-or-self" "::")
// [35] NodeTest ::= KindTest | NameTest
// [36] NameTest ::= QName | Wildcard
// [37] Wildcard ::= "*" | (NCName ":" "*") | ("*" ":" NCName)

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

			ff := func(ctx *context) (sequence, error) {
				ctx.context, err = ef(ctx)
				if err != nil {
					return nil, err
				}
				return ctx.Filter(predicate)
			}
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
		ef = func(ctx *context) (sequence, error) {
			return sequence{nexttok.Value.(string)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// NumericLiteral
	if nexttok.Typ == TokNumber {
		ef = func(ctx *context) (sequence, error) {
			return sequence{nexttok.Value.(float64)}, nil
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
		ef = func(ctx *context) (sequence, error) {
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
	ef = func(ctx *context) (sequence, error) {
		return sequence{}, nil
	}
	leaveStep(tl, "41 parsePrimaryExpr")
	return ef, nil
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

	ef = func(ctx *context) (sequence, error) {
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
		ef = func(ctx *context) (sequence, error) {
			return callFunction(functionName, []sequence{})
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
		return nil, fmt.Errorf("close paren expected")
	}
	// get expr single *
	ef = func(ctx *context) (sequence, error) {
		var arguments []sequence
		for _, es := range efs {
			seq, err := es(ctx)
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, seq)
		}

		return callFunction(functionName, arguments)
	}

	leaveStep(tl, "48 parseFunctionCall")
	return ef, nil
}

func parseXPath(tl *tokenlist) (evalFunc, error) {
	ef, err := parseExpr(tl)
	if err != nil {
		return nil, err
	}
	return ef, nil
}

// Dothings ..
func Dothings() error {
	tl, err := stringToTokenlist(` for $foo in (1,2,3) return $foo `)
	if err != nil {
		return err
	}
	fmt.Println("static parsing ------------")
	evaler, err := parseXPath(tl)
	if err != nil {
		return err
	}
	fmt.Println("dynamic evaluation ------------")
	ctx := context{}
	ctx.vars = make(map[string]sequence)
	ctx.tmpvars = make(map[string]sequence)
	ctx.vars["foo"] = sequence{"bar"}
	ctx.vars["onedotfive"] = sequence{1.5}

	seq, err := evaler(&ctx)
	if err != nil {
		return err
	}
	fmt.Println("result -----------------")
	fmt.Printf("len(seq) %#v\n", len(seq))
	fmt.Printf("seq %s\n", seq)
	return nil
}
