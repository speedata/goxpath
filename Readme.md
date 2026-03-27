# goxpath

An XPath 2.0+ evaluator written in Go with selected XPath 3.1 functions (maps, arrays, JSON).

Built on [goxml](https://github.com/speedata/goxml) for the XML tree model.

## Usage

```go
doc, _ := goxml.Parse(strings.NewReader(`<root><item id="1">Hello</item></root>`))
xp := &goxpath.Parser{Ctx: goxpath.NewContext(doc)}
result, _ := xp.Evaluate("//item[@id='1']")
fmt.Println(result) // Hello
```

See [pkg.go.dev](https://pkg.go.dev/github.com/speedata/goxpath) for the full Go API.

## Documentation

The full XPath function reference is at:

**https://doc.speedata.de/goxml/**

## Supported Functions

String, numeric, date/time, sequence, node, QName, URI, regex, JSON (`json-to-xml`, `xml-to-json`), map and array functions. See the [documentation](https://doc.speedata.de/goxml/) for the complete list.

## License

MIT — see [LICENSE](LICENSE).
