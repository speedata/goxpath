package goxpath

import (
	"fmt"

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
				if elt, ok := cld.(*goxml.Element); ok {
					copysequence := ctx.sequence
					ctx.sequence = Sequence{elt}
					s, err := ctx.descendantOrSelfAxis(tf)
					if err != nil {
						return nil, err
					}
					for _, itm := range s {
						seq = append(seq, itm)
					}
					ctx.sequence = copysequence
				}
			}
		case *goxml.Element:
			if tf(ctx, t) {
				seq = append(seq, t)
			}
			for _, cld := range t.Children() {
				if elt, ok := cld.(*goxml.Element); ok {
					copysequence := ctx.sequence
					ctx.sequence = Sequence{elt}
					s, err := ctx.descendantOrSelfAxis(tf)
					if err != nil {
						return nil, err
					}
					for _, itm := range s {
						seq = append(seq, itm)
					}
					ctx.sequence = copysequence
				} else if txt, ok := cld.(goxml.CharData); ok {
					if tf(ctx, txt) {
						seq = append(seq, txt.Contents)
					}

				}

			}
		case goxml.CharData:
			if tf(ctx, t) {
				seq = append(seq, t.Contents)
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

func (ctx *Context) followingAxis(tf testFunc) (Sequence, error) {
	for _, n := range ctx.sequence {
		switch n.(type) {
		case *goxml.XMLDocument:
			return ctx.descendantOrSelfAxis(tf)
		case *goxml.Element:
			ctx.followingSiblingAxis(tf)
			ctx.descendantOrSelfAxis(tf)

		}
	}
	return ctx.sequence, nil
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

func (ctx *Context) parentAxis(tf testFunc) (Sequence, error) {
	var seq Sequence
	for _, n := range ctx.sequence {
		switch t := n.(type) {
		case *goxml.Element:
			if tf(ctx, t.Parent) {
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

func (ctx *Context) precedingAxis(tf testFunc) (Sequence, error) {
	for _, n := range ctx.sequence {
		switch n.(type) {
		case *goxml.XMLDocument:
			return ctx.descendantOrSelfAxis(tf)
		case *goxml.Element:
			ctx.precedingSiblingAxis(tf)
			ctx.descendantOrSelfAxis(tf)

		}
	}
	return ctx.sequence, nil
}
