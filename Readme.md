# goxpath

An XPath 3.1 evaluator written in Go — **85% W3C conformance** (~19,000 of ~22,200 applicable [QT3](https://github.com/w3c/qt3tests) tests passing).

Built on [goxml](https://github.com/speedata/goxml) for the XML tree model.

## Features

- Full XPath 3.1 expression language (let, for, if, arrow, maps, arrays, inline functions, dynamic calls)
- 150+ XPath/XQuery functions including math, higher-order, JSON, date/time formatting
- Typed numeric system (xs:double, xs:float, xs:decimal, xs:integer with subtype hierarchy)
- Named function references, dynamic function calls, function-lookup
- DecimalFormat API for customizable number formatting
- Per-test regression detection against a W3C QT3 baseline

## Usage

```go
xp, _ := goxpath.NewParser(strings.NewReader(`<root><item id="1">Hello</item></root>`))
result, _ := xp.Evaluate("//item[@id='1']/text()")
fmt.Println(result) // [Hello]
```

See [pkg.go.dev](https://pkg.go.dev/github.com/speedata/goxpath) for the full Go API.

## Testing

```bash
# Run all tests including W3C QT3 conformance (~2s)
git clone --depth 1 https://github.com/w3c/qt3tests.git testdata/qt3tests
go test ./...

# Update baseline after improvements
QT3_UPDATE_BASELINE=1 go test -run TestQT3Survey
```

The test suite compares each run against `testdata/qt3_baseline.txt` (~19,000 test names). Any regression is reported as a test failure with the specific test name.

## Known Limitations

- **Regex back-references** (`\1`, `\2`) are not supported (Go RE2 engine limitation)
- **Unicode Collation Algorithm** (UCA): supported via `golang.org/x/text/collate`. `lang`, `strength`, `numeric` and `fallback` parameters are honored; `caseFirst`, `caseLevel`, `alternate`, `maxVariable`, `reorder`, `backwards`, `version`, `normalization` are accepted lax but not effectively applied (raise `FOCH0002` with `fallback=no`).
- **Integer precision** is limited to int64 (~9.2 × 10¹⁸); the spec requires arbitrary precision
- **Decimal** is stored as float64 (~15-17 significant digits)
- **Timezone handling** may add or omit timezone indicators in edge cases
- **Not implemented**: `fn:transform()`, `fn:idref()`, `namespace::` axis, schema-aware types

See the [full limitations reference](https://doc.speedata.de/goxml/xpath/limitations/) for details.

## Documentation

Full reference at **https://doc.speedata.de/goxml/** — includes language features, type system, all function categories, Go API, and known limitations.

## License

BSD-3-Clause — see [License.md](License.md).
