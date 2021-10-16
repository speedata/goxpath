package xpath

import (
	"fmt"
	"io"
	"strings"
)

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

type context struct{}

type item interface{}

// type compareFunc func(interface{}, interface{}) (bool, error)

type sequence []item

func (s sequence) String() string {
	var sb strings.Builder
	sb.WriteString(`( `)
	for _, itm := range s {
		fmt.Fprintf(&sb, "%v", itm)
	}
	sb.WriteString(` )`)
	return sb.String()
}

type evalFunc func(context) (sequence, error)

type adder func(item, item) (item, error)

func add(left, right item) (item, error) {
	var lvalue float64
	var rvalue float64
	if seq, ok := left.(sequence); ok {
		if len(seq) > 1 {
			return nil, ErrSequence
		}
		left = seq[0]
	}

	if seq, ok := right.(sequence); ok {
		if len(seq) > 1 {
			return nil, ErrSequence
		}
		right = seq[0]
	}

	switch l := left.(type) {
	case int:
		lvalue = float64(l)
	case float64:
		lvalue = l
	default:
		fmt.Printf("parse error lvalue %T\n", left)
	}

	switch r := right.(type) {
	case int:
		rvalue = float64(r)
	case float64:
		rvalue = r
	default:
		fmt.Printf("parse error rvalue %T\n", right)
	}

	a := lvalue + rvalue

	return a, nil
}

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
	f := func(ctx context) (sequence, error) {
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
	ef, err := parseExprSingle(tl)
	if err != nil {
		return nil, err
	}
	// Todo: more than one item

	f := func(ctx context) (sequence, error) {
		seq, err := ef(ctx)
		if err != nil {
			return nil, err
		}

		return seq, nil
	}
	leaveStep(tl, "2 parseExpr")
	return f, nil
}

// [3] ExprSingle ::= ForExpr | QuantifiedExpr | IfExpr | OrExpr
func parseExprSingle(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "3 parseExprSingle")
	nexttok, err := tl.read()
	if err != nil {
		return nil, err
	}
	var ef evalFunc

	if val, ok := nexttok.Value.(string); ok && val == "for" || val == "some" || val == "if" {
		switch val {
		case "for":
			// ef = parseForExpr(tl)
		case "some":
			// ef = parseQuantifiedExpr(tl)
		case "if":
			ef, err = parseIfExpr(tl)
		}
	} else {
		tl.unread()
		ef, err = parseOrExpr(tl)
	}
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "3 parseExprSingle")
	return ef, nil
}

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

	f := func(ctx context) (sequence, error) {
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
	ef, err := parseAndExpr(tl)
	if err != nil {
		return nil, err
	}
	efs = append(efs, ef)
	for {
		peek, err := tl.peek()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if peek.Value == "or" {
			tl.read()
			ef, err = parseAndExpr(tl)
			if err != nil {
				return nil, err
			}
			efs = append(efs, ef)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		return efs[0], nil
	}
	ef = func(ctx context) (sequence, error) {
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
	ef, err := parseComparisonExpr(tl)
	if err != nil {
		return nil, err
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
	peek, err := tl.peek()
	if err == io.EOF {
		return lhs, nil
	}
	if err != nil {
		return nil, err
	}

	pv := peek.Value
	if pv == "=" || pv == "<" || pv == ">" || pv == "<=" || pv == ">=" || pv == "!=" {
		tl.read()
		if rhs, err = parseRangeExpr(tl); err != nil {
			return nil, err
		}
		return doCompare(peek.Value.(string), lhs, rhs)

	}

	leaveStep(tl, "10 parseComparisonExpr")
	return lhs, nil
}

// [11] RangeExpr  ::=  AdditiveExpr ( "to" AdditiveExpr )?
func parseRangeExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "11 parseRangeExpr")
	var ef evalFunc
	ef, err := parseAdditiveExpr(tl)
	if err != nil {
		return nil, err
	}
	leaveStep(tl, "11 parseRangeExpr")
	return ef, nil
}

// [12] AdditiveExpr ::= MultiplicativeExpr ( ("+" | "-") MultiplicativeExpr )*
func parseAdditiveExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "12 parseAdditiveExpr")
	var ef evalFunc
	ef, err := parseMultiplicativeExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "12 parseAdditiveExpr")
	return ef, nil
}

// [13] MultiplicativeExpr ::=  UnionExpr ( ("*" | "div" | "idiv" | "mod") UnionExpr )*
func parseMultiplicativeExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "13 parseMultiplicativeExpr")
	var ef evalFunc
	ef, err := parseUnionExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, "13 parseMultiplicativeExpr")
	return ef, nil
}

// [14] UnionExpr ::= IntersectExceptExpr ( ("union" | "|") IntersectExceptExpr )*
func parseUnionExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, " parseUnionExpr")
	var ef evalFunc

	ef, err := parseIntersectExceptExpr(tl)
	if err != nil {
		return nil, err
	}

	leaveStep(tl, " parseUnionExpr")
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
	ef, err := parseValueExpr(tl)
	if err != nil {
		return nil, err
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
func parseFilterExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "38 parseFilterExpr")
	var ef evalFunc
	ef, err := parsePrimaryExpr(tl)
	if err != nil {
		return nil, err
	}
	leaveStep(tl, "38 parseFilterExpr")
	return ef, nil
}

// [39] PredicateList ::= Predicate*
// [40] Predicate ::= "[" Expr "]"
// [41] PrimaryExpr ::= Literal | VarRef | ParenthesizedExpr | ContextItemExpr | FunctionCall
func parsePrimaryExpr(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "41 parsePrimaryExpr")
	var ef evalFunc

	nexttok, err := tl.read()
	if err != nil {
		return nil, err
	}
	peek, err := tl.peek()
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
	} else {
		if peek.Typ == TokOpenParen {
			tl.unread()
			return parseFunctionCall(tl)
		}
	}
	ef = func(ctx context) (sequence, error) {
		return sequence{nexttok.Value}, nil
	}
	leaveStep(tl, "41 parsePrimaryExpr")
	return ef, nil
}

//  [48] FunctionCall ::= QName "(" (ExprSingle ("," ExprSingle)*)? ")"
func parseFunctionCall(tl *tokenlist) (evalFunc, error) {
	enterStep(tl, "48 parseFunctionCall")
	var ef evalFunc

	functionName, err := tl.read()
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(TokOpenParen); err != nil {
		return nil, err
	}
	peek, err := tl.peek()
	if err != nil {
		return nil, err
	}

	if peek.Typ == TokCloseParen {
		// shortcut, func()
		tl.read()
		ef = func(ctx context) (sequence, error) {
			fn := getfunction(functionName.Value.(string))
			return fn.F(sequence{}), nil
		}
	} else {
		// get expr single *
		fmt.Println(functionName)
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
	// if false {

	//  fn := "hello.xml"
	//  r, err := os.Open(fn)
	//  defer r.Close()
	//  if err != nil {
	//   return err
	//  }
	//  d, err := goxml.Parse(r)
	//  if err != nil {
	//   return err
	//  }
	//  fmt.Println("d", d.ToXML())
	// }
	tl, err := stringToTokenlist(` false() or false() `)
	if err != nil {
		return err
	}
	fmt.Println("static parsing ------------")
	evaler, err := parseXPath(tl)
	if err != nil {
		return err
	}
	fmt.Println("dynamic evaluation ------------")
	seq, err := evaler(context{})
	if err != nil {
		return err
	}
	fmt.Println("result -----------------")
	fmt.Printf("len(seq) %#v\n", len(seq))
	fmt.Printf("seq[0] %#v\n", seq[0])
	return nil
}
