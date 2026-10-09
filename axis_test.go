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

// TestEQNameNameTest checks name tests written as URIQualifiedNames
// (issue #13).
func TestEQNameNameTest(t *testing.T) {
	doc := `<root xmlns:a="urn:a" a:x="1" y="2"><a:e/><e/><f xmlns="urn:a"/></root>`
	evalStrings(t, doc, []struct{ expr, want string }{
		{"/root/@Q{urn:a}x", "1"},
		{"/root/@Q{  urn:a }x", "1"},
		{"count(/root/Q{ urn:a\n}e)", "1"},
		{"/root/attribute::Q{urn:a}x", "1"},
		{"/root/@Q{}y", "2"},
		{"count(/root/@Q{}x)", "0"},
		{"count(/root/@Q{urn:a}*)", "1"},
		{"/root/attribute(Q{urn:a}x)", "1"},
		{"count(/root/Q{urn:a}e)", "1"},
		{"count(/root/child::Q{urn:a}e)", "1"},
		{"count(/Q{}root)", "1"},
		{"count(/root/Q{}e)", "1"},
		{"count(/root/Q{urn:a}*)", "2"},
		{"/root/Q{urn:a}* ! local-name()", "e f"},
		{"count(//Q{urn:a}f)", "1"},
		{"count(/root/Q{urn:b}e)", "0"},
		{"count(/root/Q{urn:a}e[1])", "1"},
		{"/root/namespace::Q{}a", "urn:a"},
		{"count(/root/namespace::Q{urn:a}a)", "0"},
	})
}
