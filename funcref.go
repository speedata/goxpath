package goxpath

import "fmt"

// XPathFunction represents a callable function reference (XPath 3.1).
// Created by named function references (fn#arity) or inline functions.
type XPathFunction struct {
	Name             string
	Namespace        string
	Arity            int
	Fn               func(*Context, []Sequence) (Sequence, error)
	DynamicCallError string // if non-empty, calling this function reference raises this error
}

// Call invokes the function with the given arguments.
func (f *XPathFunction) Call(ctx *Context, args []Sequence) (Sequence, error) {
	if f.DynamicCallError != "" {
		return nil, NewXPathError(f.DynamicCallError, fmt.Sprintf("dynamic call to %s() is not allowed", f.Name))
	}
	if f.Arity >= 0 && len(args) != f.Arity {
		return nil, fmt.Errorf("XPTY0004: function %s#%d called with %d arguments", f.Name, f.Arity, len(args))
	}
	return f.Fn(ctx, args)
}
