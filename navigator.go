package goxpath

import (
	"fmt"
	"sort"

	"github.com/speedata/goxml"
)

func (ctx *Context) childAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.XMLDocument:
			for _, cld := range t.Children() {
				if tf(ctx, cld) {
					seq = append(seq, cld)
				}
			}
		case *goxml.Element:
			for _, cld := range t.Attributes() {
				if tf(ctx, cld) {
					seq = append(seq, cld)
				}
			}
			for _, cld := range t.Children() {
				if tf(ctx, cld) {
					if cd, ok := cld.(goxml.CharData); ok {
						seq = append(seq, cd.Contents)
					} else {
						seq = append(seq, cld)
					}
				}
			}
		case goxml.CharData:
			if tf(ctx, t) {
				seq = append(seq, t.Contents)
			}
		case goxml.Comment:
			// Comment nodes have no children.
		case goxml.ProcInst:
			// PI nodes have no children.
		case goxml.NamespaceNode:
			// Namespace nodes have no children.
		case *goxml.Attribute:
			// Attributes have no children. Without this case the
			// switch falls through to the default branch when an
			// attribute name happens to collide with a reserved
			// keyword (e.g. @float, @int) and the parser routes the
			// step through the child axis.
		case Sequence:
			for _, itm := range t {
				if tf(ctx, itm) {
					seq = append(seq, itm)
				}
			}
		case string:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		default:
			return nil, fmt.Errorf("childAxis nyi %T", t)
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) descendantOrSelfAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.XMLDocument:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
			for _, cld := range t.Children() {
				copysequence := ctx.sequence
				ctx.sequence = Sequence{cld}
				s, err := ctx.descendantOrSelfAxis(tf)
				if err != nil {
					return nil, err
				}
				seq = append(seq, s...)
				ctx.sequence = copysequence
			}
		case *goxml.Element:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
			for _, cld := range t.Children() {
				copysequence := ctx.sequence
				ctx.sequence = Sequence{cld}
				s, err := ctx.descendantOrSelfAxis(tf)
				if err != nil {
					return nil, err
				}
				seq = append(seq, s...)
				ctx.sequence = copysequence
			}
		case goxml.CharData:
			if tf(ctx, t) {
				seq = append(seq, t.Contents)
			}
		case goxml.Comment:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case goxml.ProcInst:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case goxml.NamespaceNode:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case *goxml.Attribute:
			// Attributes have no descendants; the descendant-or-self
			// axis returns just the attribute itself if it passes
			// the node test.
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case Sequence:
			for _, itm := range t {
				if tf(ctx, itm) {
					seq = append(seq, itm)
				}
			}
		default:
			return nil, fmt.Errorf("descendantOrSelfAxis nyi %T", t)
		}
	}
	ctx.sequence = seq
	return ctx.sequence, nil
}

func (ctx *Context) descendantAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.XMLDocument:
			for _, cld := range t.Children() {
				copysequence := ctx.sequence
				ctx.sequence = Sequence{cld}
				s, err := ctx.descendantAxis(tf)
				if err != nil {
					return nil, err
				}
				for _, itm := range s {
					seq = append(seq, itm)
				}
				ctx.sequence = copysequence
				if tf(ctx, cld) {
					seq = append(seq, cld)
				}
			}
		case *goxml.Element:
			for _, cld := range t.Children() {
				copysequence := ctx.sequence
				ctx.sequence = Sequence{cld}
				s, err := ctx.descendantAxis(tf)
				if err != nil {
					return nil, err
				}
				for _, itm := range s {
					seq = append(seq, itm)
				}
				ctx.sequence = copysequence
				if tf(ctx, cld) {
					seq = append(seq, cld)
				}
			}
		case goxml.CharData:
			if tf(ctx, t) {
				seq = append(seq, t.Contents)
			}
		case goxml.Comment:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case goxml.ProcInst:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case goxml.NamespaceNode:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		case *goxml.Attribute:
			// Attributes have no descendants; the descendant axis
			// (which excludes self) returns the empty sequence.
		case Sequence:
			for _, itm := range t {
				if tf(ctx, itm) {
					seq = append(seq, itm)
				}
			}
		default:
			return nil, fmt.Errorf("descendantAxis nyi %T", t)
		}
	}
	ctx.sequence = seq
	return seq, nil
}

// collectSubtree appends the descendant-or-self nodes of n that match tf
// to seq and returns the extended sequence.
func (ctx *Context) collectSubtree(seq Sequence, n goxml.XMLNode, tf testFunc) (Sequence, error) {
	copysequence := ctx.sequence
	ctx.sequence = Sequence{n}
	s, err := ctx.descendantOrSelfAxis(tf)
	ctx.sequence = copysequence
	if err != nil {
		return nil, err
	}
	return append(seq, s...), nil
}

// followingAxis returns all nodes that are after the context node in
// document order, excluding descendants. For each ancestor-or-self of the
// context node (innermost first), the following siblings and their
// subtrees are collected, which yields document order.
func (ctx *Context) followingAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	var err error
	for _, n := range ctx.sequence {
		t, ok := n.(*goxml.Element)
		if !ok {
			// The document node has no following nodes.
			continue
		}
		for e := t; e.Parent != nil; {
			after := false
			for _, cld := range e.Parent.Children() {
				if cldElt, ok := cld.(*goxml.Element); ok && cldElt == e {
					after = true
					continue
				}
				if !after {
					continue
				}
				if seq, err = ctx.collectSubtree(seq, cld, tf); err != nil {
					return nil, err
				}
			}
			pe, ok := e.Parent.(*goxml.Element)
			if !ok {
				break
			}
			e = pe
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) followingSiblingAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.XMLDocument:
			break
		case *goxml.Element:
			curid := t.ID
			for _, cld := range t.Parent.Children() {
				switch u := cld.(type) {
				case *goxml.Element:
					if u.ID > curid && tf(ctx, u) {
						seq = append(seq, u)
					}
				case goxml.CharData:
					if u.ID > curid && tf(ctx, u) {
						seq = append(seq, u)
					}
				}
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

// namespaceAxis returns a namespace node for every in-scope namespace of
// the context element, including the implicit xml namespace binding.
func (ctx *Context) namespaceAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		elt, ok := n.(*goxml.Element)
		if !ok {
			continue
		}
		prefixes := make([]string, 0, len(elt.Namespaces)+1)
		for prefix := range elt.Namespaces {
			prefixes = append(prefixes, prefix)
		}
		if _, ok := elt.Namespaces["xml"]; !ok {
			prefixes = append(prefixes, "xml")
		}
		// The spec leaves the order undefined; sort by prefix so the
		// result is deterministic.
		sort.Strings(prefixes)
		for _, prefix := range prefixes {
			uri, ok := elt.Namespaces[prefix]
			if !ok {
				uri = "http://www.w3.org/XML/1998/namespace"
			}
			nsNode := goxml.NamespaceNode{ID: goxml.NewID(), Prefix: prefix, URI: uri}
			if tf(ctx, nsNode) {
				seq = append(seq, nsNode)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) selfAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		if tf(ctx, n) {
			seq = append(seq, n)
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) parentAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.Element:
			if t.Parent != nil && tf(ctx, t.Parent) {
				seq = append(seq, t.Parent)
			}
		case *goxml.Attribute:
			if t.Parent != nil && tf(ctx, t.Parent) {
				seq = append(seq, t.Parent)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) ancestorAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.Element:
			parent := t.Parent
			if pe, ok := parent.(*goxml.Element); ok {
				newcontext := CopyContext(ctx)
				newcontext.sequence = Sequence{pe}
				s, err := newcontext.ancestorAxis(tf)
				if err != nil {
					return nil, err
				}
				for _, itm := range s {
					if tf(ctx, itm) {
						seq = append(seq, itm)
					}
				}
			}
			if tf(ctx, parent) {
				seq = append(seq, parent)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) ancestorOrSelfAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.Element:
			parent := t.Parent
			if pe, ok := parent.(*goxml.Element); ok {
				newcontext := CopyContext(ctx)
				newcontext.sequence = Sequence{pe}
				s, err := newcontext.ancestorOrSelfAxis(tf)
				if err != nil {
					return nil, err
				}
				for _, itm := range s {
					if tf(ctx, itm) {
						seq = append(seq, itm)
					}
				}
			}
			if tf(ctx, t) {
				seq = append(seq, t)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func (ctx *Context) precedingSiblingAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.XMLDocument:
			break
		case *goxml.Element:
			curid := t.ID
			for _, cld := range t.Parent.Children() {
				switch u := cld.(type) {
				case *goxml.Element:
					if u.ID < curid && tf(ctx, u) {
						seq = append(seq, u)
					}
				case goxml.CharData:
					if u.ID < curid && tf(ctx, u) {
						seq = append(seq, u)
					}
				}
			}
		}

	}
	ctx.sequence = seq
	return seq, nil
}

// precedingAxis returns all nodes that are before the context node in
// document order, excluding ancestors, attribute nodes and namespace
// nodes. For each ancestor-or-self of the context node (outermost first),
// the preceding siblings and their subtrees are collected, which yields
// document order.
func (ctx *Context) precedingAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	var err error
	for _, n := range ctx.sequence {
		t, ok := n.(*goxml.Element)
		if !ok {
			// The document node has no preceding nodes.
			continue
		}
		// Ancestor-or-self chain of the context node, outermost first.
		var chain []*goxml.Element
		for e := t; e != nil; {
			chain = append(chain, e)
			pe, ok := e.Parent.(*goxml.Element)
			if !ok {
				break
			}
			e = pe
		}
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
		for _, a := range chain {
			if a.Parent == nil {
				continue
			}
			for _, cld := range a.Parent.Children() {
				if cldElt, ok := cld.(*goxml.Element); ok && cldElt == a {
					break
				}
				if seq, err = ctx.collectSubtree(seq, cld, tf); err != nil {
					return nil, err
				}
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}
