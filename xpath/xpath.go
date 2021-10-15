package xpath

import (
	"fmt"
)

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

type context struct{}

type item interface{}

type sequence []item

type evalFunc func(context) (sequence, error)

type adder func(item, item) (item, error)

func add(left, right item) (item, error) {
	fmt.Println("call adder")
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
	fmt.Println(a)
	return a, nil
}

func getLeft(tl *tokenlist) (evalFunc, error) {
	t, err := tl.next()
	if err != nil {
		return nil, err
	}
	f := func(ctx context) (sequence, error) {
		fmt.Println("eval getLeft")
		return sequence{t.Value}, nil
	}
	return f, nil
}

func getRight(tl *tokenlist) (evalFunc, error) {
	t, err := tl.next()
	if err != nil {
		return nil, err
	}
	f := func(ctx context) (sequence, error) {
		fmt.Println("eval getRight")
		return sequence{t.Value}, nil
	}
	return f, nil
}

func parse(tl *tokenlist) (evalFunc, error) {
	var left, right evalFunc
	var err error
	var op *token
	var fn adder

	if left, err = getLeft(tl); err != nil {
		return nil, err
	}
	if op, err = tl.next(); err != nil {
		return nil, err
	}

	switch op.Value {
	case "+":
		fn = add
	default:
		fmt.Println("parse error, unknown function fn", op.Value)
	}

	if right, err = getRight(tl); err != nil {
		return nil, err
	}

	return func(ctx context) (sequence, error) {
		fmt.Println("eval parse")
		var lvalue, rvalue sequence
		var it item
		if lvalue, err = left(ctx); err != nil {
			return nil, err
		}
		if rvalue, err = right(ctx); err != nil {
			return nil, err
		}

		if it, err = fn(lvalue, rvalue); err != nil {
			return nil, err
		}
		return sequence{it}, nil
	}, nil
}

// Dothings ..
func Dothings() error {
	// if false {

	// 	fn := "hello.xml"
	// 	r, err := os.Open(fn)
	// 	defer r.Close()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	d, err := goxml.Parse(r)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	fmt.Println("d", d.ToXML())
	// }
	tl, err := stringToTokenlist(".2E-2 + 3")
	if err != nil {
		return err
	}
	fmt.Println(tl.toks)

	evaler, err := parse(tl)
	seq, err := evaler(context{})
	if err != nil {
		return err
	}
	fmt.Printf("len(seq) %#v\n", len(seq))
	fmt.Printf("seq[0] %#v\n", seq[0])
	return nil
}
