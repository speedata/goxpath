package goxpath

import (
	"strings"
	"testing"
)

func evalStrings(t *testing.T, doc string, testdata []struct{ expr, want string }) {
	t.Helper()
	p, err := NewParser(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	for _, td := range testdata {
		seq, err := p.Evaluate("string-join((" + td.expr + ") ! string(), ' ')")
		if err != nil {
			t.Errorf("%s: %v", td.expr, err)
			continue
		}
		if got := seq.Stringvalue(); got != td.want {
			t.Errorf("%s = %q, want %q", td.expr, got, td.want)
		}
	}
}

// TestChildAxisNoAttributes checks that attributes are not children
// and are only reached through the attribute axis (issue #10).
func TestChildAxisNoAttributes(t *testing.T) {
	doc := `<root><fig id="f1" position="float" float="x" int="y"><caption/>t<graphic/></fig></root>`
	evalStrings(t, doc, []struct{ expr, want string }{
		{"count(/root/fig/node())", "3"},
		{"count(/root/fig/*)", "2"},
		{"/root/fig/node()[. instance of element()] ! name()", "caption graphic"},
		{"count(/root/fig/child::attribute())", "0"},
		{"count(/root/fig/child::attribute(id))", "0"},
		{"count(/root/fig/attribute::node())", "4"},
		{"count(/root/fig/@node())", "4"},
		{"/root/fig/@* ! name()", "id position float int"},
		{"/root/fig/attribute::id", "f1"},
		{"/root/fig/@float", "x"},
		{"/root/fig/@int", "y"},
		{"count(//@*)", "4"},
		{"count(/root/fig/@id/node())", "0"},
		{"count(/root/fig/@id/@*)", "0"},
		{"/root/fig/@id/.. ! name()", "fig"},
	})
}

// TestXMLPrefix checks that the xml prefix is bound without a
// declaration (issue #11).
func TestXMLPrefix(t *testing.T) {
	doc := `<root xml:lang="en"><p xml:space="preserve" xml:id="p1"/></root>`
	evalStrings(t, doc, []struct{ expr, want string }{
		{"/root/@xml:lang", "en"},
		{"/root/p/@xml:id", "p1"},
		{"/root/@* ! name()", "xml:lang"},
		{"/root/@* ! namespace-uri()", "http://www.w3.org/XML/1998/namespace"},
		{"/root/p/@xml:* ! local-name()", "space id"},
		{"/root/@* ! prefix-from-QName(node-name())", "xml"},
	})
}

// TestAttributeKindTestDefaultAxis checks that a step with an
// attribute() kind test and no axis selects on the attribute axis.
func TestAttributeKindTestDefaultAxis(t *testing.T) {
	doc := `<root><e a="1" b="2"><attribute/></e></root>`
	evalStrings(t, doc, []struct{ expr, want string }{
		{"/root/e/attribute()", "1 2"},
		{"/root/e/attribute(b)", "2"},
		{"count(/root/e/child::attribute())", "0"},
		{"/root/e/attribute ! name()", "attribute"},
		{"/root/e/child::attribute ! name()", "attribute"},
	})
}
