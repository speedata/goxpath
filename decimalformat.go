package goxpath

// DecimalFormat defines the properties of a decimal format
// as per XPath 3.1 / XSLT 3.0 (Section 4.7.1).
// Use DefaultDecimalFormat() to create an instance with spec-default values.
type DecimalFormat struct {
	DecimalSeparator  rune   // default '.'
	GroupingSeparator rune   // default ','
	MinusSign         rune   // default '-'
	Percent           rune   // default '%'
	PerMille          rune   // default '\u2030'
	ZeroDigit         rune   // default '0'
	Digit             rune   // default '#'
	PatternSeparator  rune   // default ';'
	Infinity          string // default "Infinity"
	NaN               string // default "NaN"
	ExponentSeparator rune   // default 'e'
}

// DefaultDecimalFormat returns a DecimalFormat with the XPath spec-default values.
func DefaultDecimalFormat() *DecimalFormat {
	return &DecimalFormat{
		DecimalSeparator:  '.',
		GroupingSeparator: ',',
		MinusSign:         '-',
		Percent:           '%',
		PerMille:          '\u2030',
		ZeroDigit:         '0',
		Digit:             '#',
		PatternSeparator:  ';',
		Infinity:          "Infinity",
		NaN:               "NaN",
		ExponentSeparator: 'e',
	}
}

// SetDecimalFormat registers a named decimal format on the context.
// Use name "" for the default (unnamed) format.
// For qualified names, use "namespace-uri localname" as key.
func (ctx *Context) SetDecimalFormat(name string, df *DecimalFormat) {
	if ctx.decimalFormats == nil {
		ctx.decimalFormats = make(map[string]*DecimalFormat)
	}
	ctx.decimalFormats[name] = df
}

// GetDecimalFormat returns the decimal format with the given name.
// Returns the default format for name "".
// Returns nil and an error for unknown named formats (FODF1280).
func (ctx *Context) GetDecimalFormat(name string) (*DecimalFormat, error) {
	if ctx.decimalFormats != nil {
		if df, ok := ctx.decimalFormats[name]; ok {
			return df, nil
		}
	}
	if name == "" {
		return DefaultDecimalFormat(), nil
	}
	return nil, NewXPathError("FODF1280", "unknown decimal format: "+name)
}
