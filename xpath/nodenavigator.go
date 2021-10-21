package xpath

import (
	"fmt"

	"github.com/speedata/goxml"
)

// NodeNavigator moves around a tree
type NodeNavigator struct {
	current []goxml.XMLNode
	xmldoc  *goxml.XMLDocument
	ctx     *Context
}

// NewNodeNavigator returns a new node navigator from the xml document
func NewNodeNavigator(doc *goxml.XMLDocument) *NodeNavigator {
	return &NodeNavigator{
		current: nil,
		xmldoc:  doc,
	}
}

// Document moves the node navigator to the document and retuns it
func (nn *NodeNavigator) Document() goxml.XMLNode {
	nn.current = []goxml.XMLNode{nn.xmldoc}
	return nn.xmldoc
}

// Root moves the node navigator to the root node of the document
func (nn *NodeNavigator) Root() (goxml.XMLNode, error) {
	var err error
	cur, err := nn.xmldoc.Root()
	nn.current = []goxml.XMLNode{cur}
	return nn.xmldoc, err
}

type testfuncChildren func(*goxml.Element) bool
type testfuncAttributes func(goxml.XMLNode) bool

// Child returns all children of the current node that satisfy the testfunc
func (nn *NodeNavigator) Child(tf testfuncChildren, ctx *Context) ([]goxml.XMLNode, error) {
	fmt.Println("nn/Child")
	var nodes []goxml.XMLNode
	for _, n := range nn.current {
		for _, c := range n.Children() {
			if celt, ok := c.(*goxml.Element); ok {
				if tf(celt) {
					nodes = append(nodes, c)
				}
			}
		}
	}
	fmt.Println(nodes)
	nn.current = nodes
	var seq Sequence
	for _, n := range nodes {
		seq = append(seq, n)
	}
	ctx.context = seq
	return nodes, nil
}

func returnIsNameTF(name string) testfuncChildren {
	tf := func(elt *goxml.Element) bool {
		if elt.Name == name {
			return true
		}
		return false
	}
	return tf
}
