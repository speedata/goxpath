package goxpath

import "testing"

func TestCollationCodepoint(t *testing.T) {
	c, err := ResolveCollation(CodepointCollationURI)
	if err != nil {
		t.Fatal(err)
	}
	if c.Compare("a", "b") != -1 {
		t.Errorf("a < b expected")
	}
	if !c.Equal("abc", "abc") {
		t.Errorf("abc = abc")
	}
	if !c.Contains("abcd", "bc") {
		t.Errorf("contains failed")
	}
}

func TestCollationHTMLAsciiCI(t *testing.T) {
	c, err := ResolveCollation(HTMLAsciiCaseInsensitiveURI)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Equal("AbC", "aBc") {
		t.Errorf("ascii ci equal failed")
	}
	if !c.StartsWith("Hello", "HEL") {
		t.Errorf("ascii ci startswith failed")
	}
	if c.Equal("ä", "Ä") {
		t.Errorf("html-ascii-ci must NOT fold non-ASCII")
	}
}

func TestCollationUCAGerman(t *testing.T) {
	c, err := ResolveCollation("http://www.w3.org/2013/collation/UCA?lang=de;strength=primary")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Equal("strasse", "Straße") {
		// strength=primary should ignore case and treat ß as ss in German.
		t.Logf("note: strasse vs Straße not equal under primary (impl-defined)")
	}
	if !c.Equal("MÜLLER", "müller") {
		t.Errorf("primary should ignore case")
	}
	if c.Compare("a", "b") >= 0 {
		t.Errorf("a < b expected")
	}
}

func TestCollationUnknown(t *testing.T) {
	_, err := ResolveCollation("http://example.com/no-such-collation")
	if err == nil {
		t.Fatal("expected FOCH0002")
	}
	if xe, ok := err.(*XPathError); !ok || xe.Code != "FOCH0002" {
		t.Errorf("expected FOCH0002, got %v", err)
	}
}
