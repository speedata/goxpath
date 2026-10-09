package goxpath

import "testing"

// TestDescendantAxis checks that the descendant axis returns every node
// once, in document order, and nothing for nodes without children
// (issue #12).
func TestDescendantAxis(t *testing.T) {
	doc := `<root><a><b>t1</b><!--c--><?pi x?>t2</a></root>`
	evalStrings(t, doc, []struct{ expr, want string }{
		{"/root/descendant::node() ! (if (. instance of element()) then name() else string(.))", "a b t1 c x t2"},
		{"/root/descendant::text()", "t1 t2"},
		{"count(/root/descendant::comment())", "1"},
		{"count(/root/descendant::processing-instruction())", "1"},
		{"/root/descendant::node()[2] ! name()", "b"},
		{"/root/descendant::*[1] ! name()", "a"},
		{"count(/descendant::node())", "7"},
		{"count(/root/a/comment()/descendant::node())", "0"},
		{"count(/root/a/processing-instruction()/descendant::node())", "0"},
		{"count(/root/descendant::node()) = count(/root//node())", "true"},
	})
}
