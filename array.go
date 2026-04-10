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
	RegisterFunction(&Function{Name: "head", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		if len(arr.Members) == 0 {
			return nil, fmt.Errorf("FOAY0001: array is empty")
		}
		return arr.Members[0], nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "tail", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		if len(arr.Members) == 0 {
			return nil, fmt.Errorf("FOAY0001: array is empty")
		}
		return Sequence{&XPathArray{Members: arr.Members[1:]}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "append", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		newMembers := make([]Sequence, len(arr.Members)+1)
		copy(newMembers, arr.Members)
		newMembers[len(arr.Members)] = args[1]
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "subarray", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		start, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		s := max(
			// 1-based to 0-based
			int(start)-1, 0)

		length := len(arr.Members) - s
		if len(args) > 2 {
			l, err := NumberValue(args[2])
			if err != nil {
				return nil, err
			}
			length = int(l)
		}
		end := min(s+length, len(arr.Members))
		if s >= len(arr.Members) || end <= s {
			return Sequence{&XPathArray{}}, nil
		}
		return Sequence{&XPathArray{Members: arr.Members[s:end]}}, nil
	}, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "remove", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		pos, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		idx := int(pos) - 1
		if idx < 0 || idx >= len(arr.Members) {
			return nil, fmt.Errorf("FOAY0001: index %d out of bounds", int(pos))
		}
		newMembers := make([]Sequence, 0, len(arr.Members)-1)
		newMembers = append(newMembers, arr.Members[:idx]...)
		newMembers = append(newMembers, arr.Members[idx+1:]...)
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "insert-before", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		pos, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		idx := min(max(int(pos)-1, 0), len(arr.Members))
		newMembers := make([]Sequence, 0, len(arr.Members)+1)
		newMembers = append(newMembers, arr.Members[:idx]...)
		newMembers = append(newMembers, args[2])
		newMembers = append(newMembers, arr.Members[idx:]...)
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "put", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		pos, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		idx := int(pos) - 1
		if idx < 0 || idx >= len(arr.Members) {
			return nil, fmt.Errorf("FOAY0001: index %d out of bounds", int(pos))
		}
		newMembers := make([]Sequence, len(arr.Members))
		copy(newMembers, arr.Members)
		newMembers[idx] = args[2]
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "reverse", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		newMembers := make([]Sequence, len(arr.Members))
		for i, m := range arr.Members {
			newMembers[len(arr.Members)-1-i] = m
		}
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "join", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		var allMembers []Sequence
		for _, item := range args[0] {
			arr, ok := item.(*XPathArray)
			if !ok {
				return nil, fmt.Errorf("array:join: expected array, got %T", item)
			}
			allMembers = append(allMembers, arr.Members...)
		}
		return Sequence{&XPathArray{Members: allMembers}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "flatten", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		return flattenSequence(args[0]), nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "for-each", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		fn, ok := args[1][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("array:for-each: second argument must be a function")
		}
		newMembers := make([]Sequence, len(arr.Members))
		for i, member := range arr.Members {
			res, err := fn.Call(ctx, []Sequence{member})
			if err != nil {
				return nil, err
			}
			newMembers[i] = res
		}
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "filter", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		fn, ok := args[1][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("array:filter: second argument must be a function")
		}
		var newMembers []Sequence
		for _, member := range arr.Members {
			res, err := fn.Call(ctx, []Sequence{member})
			if err != nil {
				return nil, err
			}
			if len(res) == 1 {
				if b, ok := res[0].(bool); ok && b {
					newMembers = append(newMembers, member)
				}
			}
		}
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "sort", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		// Simple sort by string value of each member
		newMembers := make([]Sequence, len(arr.Members))
		copy(newMembers, arr.Members)
		// TODO: support collation and key function arguments
		return Sequence{&XPathArray{Members: newMembers}}, nil
	}, MinArg: 1, MaxArg: 3})
	RegisterFunction(&Function{Name: "fold-left", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		fn, ok := args[2][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("array:fold-left: third argument must be a function")
		}
		acc := args[1]
		for _, member := range arr.Members {
			acc, err = fn.Call(ctx, []Sequence{acc, member})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "fold-right", Namespace: nsArray, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		arr, err := asArray(args[0])
		if err != nil {
			return nil, err
		}
		fn, ok := args[2][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("array:fold-right: third argument must be a function")
		}
		acc := args[1]
		for i := len(arr.Members) - 1; i >= 0; i-- {
			acc, err = fn.Call(ctx, []Sequence{arr.Members[i], acc})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, MinArg: 3, MaxArg: 3})
}

func asArray(seq Sequence) (*XPathArray, error) {
	if len(seq) != 1 {
		return nil, fmt.Errorf("expected a single array")
	}
	arr, ok := seq[0].(*XPathArray)
	if !ok {
		return nil, fmt.Errorf("expected an array, got %T", seq[0])
	}
	return arr, nil
}

func flattenSequence(seq Sequence) Sequence {
	var result Sequence
	for _, item := range seq {
		if arr, ok := item.(*XPathArray); ok {
			for _, member := range arr.Members {
				result = append(result, flattenSequence(member)...)
			}
		} else {
			result = append(result, item)
		}
	}
	return result
}
