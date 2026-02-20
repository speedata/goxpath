package goxpath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speedata/goxml"
)

func TestFnDoc(t *testing.T) {
	// Create a temp directory and XML file
	tmpDir := t.TempDir()
	extFile := filepath.Join(tmpDir, "external.xml")
	err := os.WriteFile(extFile, []byte(`<data><item>hello</item></data>`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a base XML document for the parser
	sr := strings.NewReader("<root/>")
	np, err := NewParser(sr)
	if err != nil {
		t.Fatal(err)
	}

	// Set the baseURI in Store so relative paths resolve
	baseURI := filepath.Join(tmpDir, "base.xsl")
	np.Ctx.Store = map[interface{}]interface{}{
		"baseURI": baseURI,
	}

	// Test 1: Load document with relative path
	seq, err := np.Evaluate(`doc('external.xml')`)
	if err != nil {
		t.Fatalf("doc('external.xml') failed: %v", err)
	}
	if len(seq) != 1 {
		t.Fatalf("expected 1 item, got %d", len(seq))
	}
	xmlDoc, ok := seq[0].(*goxml.XMLDocument)
	if !ok {
		t.Fatalf("expected *goxml.XMLDocument, got %T", seq[0])
	}

	// Navigate into the document: /data/item should contain "hello"
	np2 := &Parser{Ctx: NewContext(xmlDoc)}
	seq2, err := np2.Evaluate(`string(/data/item)`)
	if err != nil {
		t.Fatalf("navigating doc() result failed: %v", err)
	}
	if len(seq2) != 1 || seq2[0] != "hello" {
		t.Fatalf("expected 'hello', got %v", seq2)
	}

	// Test 2: Load document with absolute path
	np3, err := NewParser(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatal(err)
	}
	np3.Ctx.Store = map[interface{}]interface{}{}

	seq3, err := np3.Evaluate(`doc('` + extFile + `')`)
	if err != nil {
		t.Fatalf("doc(absolute) failed: %v", err)
	}
	if len(seq3) != 1 {
		t.Fatalf("expected 1 item, got %d", len(seq3))
	}

	// Test 3: Empty argument returns empty sequence
	np4, err := NewParser(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatal(err)
	}
	seq4, err := np4.Evaluate(`doc(())`)
	if err != nil {
		t.Fatalf("doc(()) failed: %v", err)
	}
	if len(seq4) != 0 {
		t.Fatalf("expected empty sequence, got %d items", len(seq4))
	}

	// Test 4: Caching - loading the same document twice returns same object
	np5, err := NewParser(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatal(err)
	}
	np5.Ctx.Store = map[interface{}]interface{}{
		"baseURI": baseURI,
	}

	seq5a, err := np5.Evaluate(`doc('external.xml')`)
	if err != nil {
		t.Fatalf("first doc() call failed: %v", err)
	}
	seq5b, err := np5.Evaluate(`doc('external.xml')`)
	if err != nil {
		t.Fatalf("second doc() call failed: %v", err)
	}
	if seq5a[0] != seq5b[0] {
		t.Fatal("expected same cached document, got different objects")
	}

	// Test 5: Non-existent file returns error
	np6, err := NewParser(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = np6.Evaluate(`doc('nonexistent.xml')`)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
