package goxpath

import "fmt"

const nsArray = "http://www.w3.org/2005/xpath-functions/array"

// XPathArray represents an XPath 3.1 array.
type XPathArray struct {
	Members []Sequence
}

// Get returns the member at the given 1-based index.
func (a *XPathArray) Get(pos int) (Sequence, error) {
	if pos < 1 || pos > len(a.Members) {
		return nil, fmt.Errorf("array index %d out of bounds (size %d)", pos, len(a.Members))
	}
	return a.Members[pos-1], nil
}

// Size returns the number of members in the array.
func (a *XPathArray) Size() int {
	return len(a.Members)
}

func fnArrayGet(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("array:get expects a single array as first argument")
	}
	arr, ok := args[0][0].(*XPathArray)
	if !ok {
		return nil, fmt.Errorf("array:get expects an array as first argument, got %T", args[0][0])
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("array:get expects a single integer as second argument")
	}
	pos, err := NumberValue(args[1])
	if err != nil {
		return nil, err
	}
	return arr.Get(int(pos))
}

func fnArraySize(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("array:size expects a single array as first argument")
	}
	arr, ok := args[0][0].(*XPathArray)
	if !ok {
		return nil, fmt.Errorf("array:size expects an array as first argument, got %T", args[0][0])
	}
	return Sequence{arr.Size()}, nil
}

func init() {
	RegisterFunction(&Function{Name: "get", Namespace: nsArray, F: fnArrayGet, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "size", Namespace: nsArray, F: fnArraySize, MinArg: 1, MaxArg: 1})
}
