package goxpath

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrConversion is returned in case of an unsuccessful cast.
var ErrConversion = fmt.Errorf("conversion failed")

// XSAnyURI represents an xs:anyURI value.
type XSAnyURI string

// XSUntypedAtomic represents an xs:untypedAtomic value.
type XSUntypedAtomic string

// XSHexBinary represents an xs:hexBinary value (uppercase hex string).
type XSHexBinary string

// XSBase64Binary represents an xs:base64Binary value.
type XSBase64Binary string

// XSDouble represents an xs:double value (IEEE 754 double precision).
type XSDouble float64

// XSFloat represents an xs:float value (IEEE 754 single precision, stored as float64).
type XSFloat float64

// XSDecimal represents an xs:decimal value (no INF, no NaN).
type XSDecimal float64

// IntSubtype identifies the specific integer subtype in the XSD type hierarchy.
type IntSubtype uint8

const (
	IntInteger            IntSubtype = iota // xs:integer
	IntLong                                 // xs:long
	IntInt                                  // xs:int
	IntShort                                // xs:short
	IntByte                                 // xs:byte
	IntNonNegativeInteger                   // xs:nonNegativeInteger
	IntUnsignedLong                         // xs:unsignedLong
	IntUnsignedInt                          // xs:unsignedInt
	IntUnsignedShort                        // xs:unsignedShort
	IntUnsignedByte                         // xs:unsignedByte
	IntPositiveInteger                      // xs:positiveInteger
	IntNonPositiveInteger                   // xs:nonPositiveInteger
	IntNegativeInteger                      // xs:negativeInteger
)

// intParent encodes the XSD integer type hierarchy.
// Each subtype's parent is the next type up in the hierarchy.
var intParent = [...]IntSubtype{
	IntInteger:            IntInteger, // root
	IntLong:               IntInteger,
	IntInt:                IntLong,
	IntShort:              IntInt,
	IntByte:               IntShort,
	IntNonNegativeInteger: IntInteger,
	IntUnsignedLong:       IntNonNegativeInteger,
	IntUnsignedInt:        IntUnsignedLong,
	IntUnsignedShort:      IntUnsignedInt,
	IntUnsignedByte:       IntUnsignedShort,
	IntPositiveInteger:    IntNonNegativeInteger,
	IntNonPositiveInteger: IntInteger,
	IntNegativeInteger:    IntNonPositiveInteger,
}

// intSubtypeName maps IntSubtype to XSD type name.
var intSubtypeName = [...]string{
	IntInteger:            "xs:integer",
	IntLong:               "xs:long",
	IntInt:                "xs:int",
	IntShort:              "xs:short",
	IntByte:               "xs:byte",
	IntNonNegativeInteger: "xs:nonNegativeInteger",
	IntUnsignedLong:       "xs:unsignedLong",
	IntUnsignedInt:        "xs:unsignedInt",
	IntUnsignedShort:      "xs:unsignedShort",
	IntUnsignedByte:       "xs:unsignedByte",
	IntPositiveInteger:    "xs:positiveInteger",
	IntNonPositiveInteger: "xs:nonPositiveInteger",
	IntNegativeInteger:    "xs:negativeInteger",
}

// IntIsSubtypeOf returns true if child is the same as or a subtype of ancestor.
func IntIsSubtypeOf(child, ancestor IntSubtype) bool {
	for {
		if child == ancestor {
			return true
		}
		parent := intParent[child]
		if parent == child {
			return false // reached root without finding ancestor
		}
		child = parent
	}
}

// XSInteger represents an xs:integer value with subtype information.
type XSInteger struct {
	V       int
	Subtype IntSubtype
}

// StrSubtype identifies the specific string subtype in the XSD type hierarchy.
type StrSubtype uint8

const (
	StrString           StrSubtype = iota // xs:string
	StrNormalizedString                   // xs:normalizedString
	StrToken                              // xs:token
	StrLanguage                           // xs:language
	StrNMTOKEN                            // xs:NMTOKEN
	StrName                               // xs:Name
	StrNCName                             // xs:NCName
	StrID                                 // xs:ID
	StrIDREF                              // xs:IDREF
	StrENTITY                             // xs:ENTITY
)

// strParent encodes the XSD string type hierarchy.
var strParent = [...]StrSubtype{
	StrString:           StrString, // root
	StrNormalizedString: StrString,
	StrToken:            StrNormalizedString,
	StrLanguage:         StrToken,
	StrNMTOKEN:          StrToken,
	StrName:             StrToken,
	StrNCName:           StrName,
	StrID:               StrNCName,
	StrIDREF:            StrNCName,
	StrENTITY:           StrNCName,
}

// strSubtypeName maps StrSubtype to XSD type name.
var strSubtypeName = [...]string{
	StrString:           "xs:string",
	StrNormalizedString: "xs:normalizedString",
	StrToken:            "xs:token",
	StrLanguage:         "xs:language",
	StrNMTOKEN:          "xs:NMTOKEN",
	StrName:             "xs:Name",
	StrNCName:           "xs:NCName",
	StrID:               "xs:ID",
	StrIDREF:            "xs:IDREF",
	StrENTITY:           "xs:ENTITY",
}

// StrIsSubtypeOf returns true if child is the same as or a subtype of ancestor.
func StrIsSubtypeOf(child, ancestor StrSubtype) bool {
	for {
		if child == ancestor {
			return true
		}
		parent := strParent[child]
		if parent == child {
			return false
		}
		child = parent
	}
}

// XSString represents an xs:string value with subtype information.
type XSString struct {
	V       string
	Subtype StrSubtype
}

// ToFloat64 extracts the float64 value from any numeric item.
// Returns the value and true if the item is numeric, or 0 and false otherwise.
func ToFloat64(itm any) (float64, bool) {
	switch v := itm.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case XSInteger:
		return float64(v.V), true
	case XSDouble:
		return float64(v), true
	case XSFloat:
		return float64(v), true
	case XSDecimal:
		return float64(v), true
	}
	return 0, false
}

// NumericTypeID identifies the XPath numeric type of an item.
type NumericTypeID int

const (
	NumInteger NumericTypeID = iota
	NumDecimal
	NumFloat
	NumDouble
	NumUnknown
)

// NumericType returns the numeric type ID for an item.
func NumericType(itm any) NumericTypeID {
	switch itm.(type) {
	case int:
		return NumInteger
	case XSInteger:
		return NumInteger
	case XSDecimal:
		return NumDecimal
	case XSFloat:
		return NumFloat
	case XSDouble:
		return NumDouble
	case float64:
		return NumDouble // bare float64 treated as double
	}
	return NumUnknown
}

// PromoteNumeric returns the promoted type for a binary numeric operation.
// XPath type promotion: integer < decimal < float < double.
func PromoteNumeric(a, b NumericTypeID) NumericTypeID {
	if a > b {
		return a
	}
	return b
}

// WrapNumeric wraps a float64 result in the appropriate numeric type.
func WrapNumeric(val float64, nt NumericTypeID) Item {
	switch nt {
	case NumInteger:
		return XSInteger{V: int(val), Subtype: IntInteger}
	case NumDecimal:
		return XSDecimal(val)
	case NumFloat:
		return XSFloat(val)
	case NumDouble:
		return XSDouble(val)
	}
	return val
}

// XSDuration represents an xs:duration value with separate date and time components.
type XSDuration struct {
	Negative bool
	Years    int
	Months   int
	Days     int
	Hours    int
	Minutes  int
	Seconds  float64
}

func (d XSDuration) String() string {
	// Normalize: carry months→years, seconds→minutes→hours→days
	years := d.Years + d.Months/12
	months := d.Months % 12

	totalSec := d.Seconds
	minutes := d.Minutes + int(totalSec)/60
	secs := totalSec - float64(int(totalSec)/60*60)
	hours := d.Hours + minutes/60
	minutes = minutes % 60
	days := d.Days + hours/24
	hours = hours % 24

	var sb strings.Builder
	if d.Negative {
		sb.WriteByte('-')
	}
	sb.WriteByte('P')
	if years != 0 {
		fmt.Fprintf(&sb, "%dY", years)
	}
	if months != 0 {
		fmt.Fprintf(&sb, "%dM", months)
	}
	if days != 0 {
		fmt.Fprintf(&sb, "%dD", days)
	}
	if hours != 0 || minutes != 0 || secs != 0 {
		sb.WriteByte('T')
		if hours != 0 {
			fmt.Fprintf(&sb, "%dH", hours)
		}
		if minutes != 0 {
			fmt.Fprintf(&sb, "%dM", minutes)
		}
		if secs != 0 {
			s := strconv.FormatFloat(secs, 'f', -1, 64)
			sb.WriteString(s)
			sb.WriteByte('S')
		}
	}
	if sb.Len() == 1 || (d.Negative && sb.Len() == 2) {
		sb.WriteString("T0S")
	}
	return sb.String()
}

// ParseXSDuration parses an ISO 8601 duration string (e.g. "P1Y2M3DT4H5M6.5S", "-P1Y").
func ParseXSDuration(s string) (XSDuration, error) {
	var d XSDuration
	orig := s
	if len(s) == 0 {
		return d, NewXPathError("FORG0001", "empty duration string")
	}
	if s[0] == '-' {
		d.Negative = true
		s = s[1:]
	}
	if len(s) == 0 || s[0] != 'P' {
		return d, NewXPathError("FORG0001", fmt.Sprintf("duration must start with 'P': %q", orig))
	}
	s = s[1:]
	inTimePart := false
	for len(s) > 0 {
		if s[0] == 'T' {
			inTimePart = true
			s = s[1:]
			continue
		}
		// Read a number (possibly decimal)
		numEnd := 0
		for numEnd < len(s) && (s[numEnd] >= '0' && s[numEnd] <= '9' || s[numEnd] == '.') {
			numEnd++
		}
		if numEnd == 0 || numEnd >= len(s) {
			return d, NewXPathError("FORG0001", fmt.Sprintf("invalid duration format: %q", orig))
		}
		numStr := s[:numEnd]
		designator := s[numEnd]
		s = s[numEnd+1:]

		// Validate: number must start with a digit, not just "."
		if numStr[0] == '.' || numStr[len(numStr)-1] == '.' {
			return d, NewXPathError("FORG0001", fmt.Sprintf("invalid number in duration: %q", orig))
		}

		if designator == 'S' || strings.Contains(numStr, ".") {
			f, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return d, NewXPathError("FORG0001", fmt.Sprintf("invalid number in duration: %q", orig))
			}
			if designator == 'S' {
				d.Seconds = f
			} else {
				return d, NewXPathError("FORG0001", fmt.Sprintf("decimal only allowed for seconds: %q", orig))
			}
		} else {
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return d, NewXPathError("FORG0001", fmt.Sprintf("invalid number in duration: %q", orig))
			}
			switch designator {
			case 'Y':
				d.Years = n
			case 'M':
				if inTimePart {
					d.Minutes = n
				} else {
					d.Months = n
				}
			case 'D':
				d.Days = n
			case 'H':
				d.Hours = n
			default:
				return d, NewXPathError("FORG0001", fmt.Sprintf("unexpected designator '%c' in duration: %q", designator, orig))
			}
		}
	}
	return d, nil
}

// XSGYear represents xs:gYear.
type XSGYear string

// XSGMonth represents xs:gMonth.
type XSGMonth string

// XSGDay represents xs:gDay.
type XSGDay string

// XSGYearMonth represents xs:gYearMonth.
type XSGYearMonth string

// XSGMonthDay represents xs:gMonthDay.
type XSGMonthDay string

func xsGYear(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	// Basic validation: must look like a year (optional -, 4+ digits, optional timezone)
	if !regexp.MustCompile(`^-?\d{4,}(Z|[+-]\d{2}:\d{2})?$`).MatchString(sv) {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gYear", sv))
	}
	return Sequence{XSGYear(sv)}, nil
}

func xsGMonth(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	if !regexp.MustCompile(`^--\d{2}(Z|[+-]\d{2}:\d{2})?$`).MatchString(sv) {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gMonth", sv))
	}
	month, _ := strconv.Atoi(sv[2:4])
	if month < 1 || month > 12 {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gMonth: invalid month", sv))
	}
	return Sequence{XSGMonth(sv)}, nil
}

func xsGDay(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	if !regexp.MustCompile(`^---\d{2}(Z|[+-]\d{2}:\d{2})?$`).MatchString(sv) {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gDay", sv))
	}
	day, _ := strconv.Atoi(sv[3:5])
	if day < 1 || day > 31 {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gDay: invalid day", sv))
	}
	return Sequence{XSGDay(sv)}, nil
}

func xsGYearMonth(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	if !regexp.MustCompile(`^-?\d{4,}-\d{2}(Z|[+-]\d{2}:\d{2})?$`).MatchString(sv) {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gYearMonth", sv))
	}
	// Extract and validate month
	parts := regexp.MustCompile(`^(-?\d{4,})-(\d{2})`).FindStringSubmatch(sv)
	if len(parts) >= 3 {
		month, _ := strconv.Atoi(parts[2])
		if month < 1 || month > 12 {
			return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gYearMonth: invalid month", sv))
		}
	}
	return Sequence{XSGYearMonth(sv)}, nil
}

func xsGMonthDay(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	// Must be --MM-DD format with valid month (01-12) and day (01-31)
	if !regexp.MustCompile(`^--\d{2}-\d{2}(Z|[+-]\d{2}:\d{2})?$`).MatchString(sv) {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gMonthDay", sv))
	}
	month, _ := strconv.Atoi(sv[2:4])
	day, _ := strconv.Atoi(sv[5:7])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gMonthDay: invalid month/day", sv))
	}
	// Month-specific day limits
	maxDays := [...]int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if day > maxDays[month] {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:gMonthDay: day %d exceeds max for month %d", sv, day, month))
	}
	return Sequence{XSGMonthDay(sv)}, nil
}

// XSQName represents an xs:QName value with namespace URI, prefix, and local name.
type XSQName struct {
	Namespace string
	Prefix    string
	Localname string
}

func (q XSQName) String() string {
	if q.Prefix != "" {
		return q.Prefix + ":" + q.Localname
	}
	return q.Localname
}

// ToXSInteger converts the item to an xs:integer.
func ToXSInteger(itm Item) (int, error) {
	switch t := itm.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case XSInteger:
		return t.V, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %q to xs:integer", t)
		}
		return int(f), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to xs:integer", itm)
	}
}

func xsTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	firstarg = strings.TrimSpace(firstarg)

	formats := []string{
		"15:04:05.999999999-07:00",
		"15:04:05.999999999Z",
		"15:04:05.999999999",
		"15:04:05-07:00",
		"15:04:05Z",
		"15:04:05",
	}
	for _, format := range formats {
		t, err := time.Parse(format, firstarg)
		if err == nil {
			return Sequence{XSTime(t)}, nil
		}
	}
	return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:time", firstarg))
}

func xsDouble(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	item := args[0][0]
	if f, ok := ToFloat64(item); ok {
		return Sequence{XSDouble(f)}, nil
	}
	if b, ok := item.(bool); ok {
		if b {
			return Sequence{XSDouble(1)}, nil
		}
		return Sequence{XSDouble(0)}, nil
	}
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	// Handle XPath-specific INF/NaN (case-sensitive per spec)
	switch sv {
	case "INF":
		return Sequence{XSDouble(math.Inf(1))}, nil
	case "-INF":
		return Sequence{XSDouble(math.Inf(-1))}, nil
	case "NaN":
		return Sequence{XSDouble(math.NaN())}, nil
	}
	// Reject case variants that Go's ParseFloat would accept
	upper := strings.ToUpper(sv)
	if strings.Contains(upper, "INF") || upper == "NAN" || upper == "-NAN" || upper == "+NAN" {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:double", sv))
	}
	f, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:double", sv))
	}
	return Sequence{XSDouble(f)}, nil
}

func xsDate(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}

	formats := []string{
		"2006-01-02-07:00",
		"2006-01-02+07:00",
		"2006-01-02Z",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, firstarg)
		if err == nil {
			return Sequence{XSDate(t)}, nil
		}
	}

	return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:date", firstarg))
}

// xsIntegerTyped returns a constructor function for a specific integer subtype.
// intRanges defines value ranges for integer subtypes (min, max).
// Types not in this map have no range restriction.
var intRanges = map[IntSubtype][2]int64{
	IntByte:               {-128, 127},
	IntShort:              {-32768, 32767},
	IntInt:                {-2147483648, 2147483647},
	IntLong:               {-9223372036854775808, 9223372036854775807},
	IntUnsignedByte:       {0, 255},
	IntUnsignedShort:      {0, 65535},
	IntUnsignedInt:        {0, 4294967295},
	IntUnsignedLong:       {0, 9223372036854775807}, // Go int is int64
	IntPositiveInteger:    {1, 9223372036854775807},
	IntNonNegativeInteger: {0, 9223372036854775807},
	IntNonPositiveInteger: {-9223372036854775808, 0},
	IntNegativeInteger:    {-9223372036854775808, -1},
}

func xsIntegerTyped(subtype IntSubtype) func(*Context, []Sequence) (Sequence, error) {
	return func(ctx *Context, args []Sequence) (Sequence, error) {
		seq, err := xsInteger(ctx, args)
		if err != nil {
			return nil, err
		}
		if len(seq) == 0 {
			return seq, nil
		}
		var v int
		if bare, ok := seq[0].(int); ok {
			v = bare
		} else if xi, ok := seq[0].(XSInteger); ok {
			v = xi.V
		} else {
			return seq, nil
		}
		// Range validation
		if rng, ok := intRanges[subtype]; ok {
			if int64(v) < rng[0] || int64(v) > rng[1] {
				return nil, NewXPathError("FORG0001",
					fmt.Sprintf("value %d out of range for %s", v, intSubtypeName[subtype]))
			}
		}
		return Sequence{XSInteger{V: v, Subtype: subtype}}, nil
	}
}

// xsStringTyped returns a constructor function for a specific string subtype.
func xsStringTyped(subtype StrSubtype) func(*Context, []Sequence) (Sequence, error) {
	return func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		// Validate string subtypes
		if err := validateStringSubtype(sv, subtype); err != nil {
			return nil, err
		}
		return Sequence{XSString{V: sv, Subtype: subtype}}, nil
	}
}

// validateStringSubtype checks that a string value conforms to the XSD subtype constraints.
func validateStringSubtype(sv string, subtype StrSubtype) error {
	switch subtype {
	case StrLanguage:
		// BCP 47 language tag: [a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*
		if !regexp.MustCompile(`^[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*$`).MatchString(sv) {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:language: %q", sv))
		}
	case StrNMTOKEN:
		// NMTOKEN: one or more name characters
		if sv == "" || !regexp.MustCompile(`^[\w.\-:]+$`).MatchString(sv) {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:NMTOKEN: %q", sv))
		}
	case StrName:
		// XML Name: starts with letter or _, followed by name chars
		if sv == "" || !regexp.MustCompile(`^[a-zA-Z_][\w.\-:]*$`).MatchString(sv) {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:Name: %q", sv))
		}
	case StrNCName, StrID, StrIDREF, StrENTITY:
		// NCName: Name without colons
		if sv == "" || !regexp.MustCompile(`^[a-zA-Z_][\w.\-]*$`).MatchString(sv) {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:NCName: %q", sv))
		}
	case StrToken:
		// Token: no leading/trailing whitespace, no internal sequences of spaces
		if sv != strings.TrimSpace(sv) || strings.Contains(sv, "  ") || strings.ContainsAny(sv, "\t\n\r") {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:token: %q", sv))
		}
	case StrNormalizedString:
		// NormalizedString: no tabs, newlines, or carriage returns
		if strings.ContainsAny(sv, "\t\n\r") {
			return NewXPathError("FORG0001", fmt.Sprintf("invalid xs:normalizedString: %q", sv))
		}
	}
	return nil
}

func xsInteger(_ *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	item := args[0][0]
	// Boolean → integer: true=1, false=0
	if b, ok := item.(bool); ok {
		if b {
			return Sequence{1}, nil
		}
		return Sequence{0}, nil
	}
	// Direct numeric conversion
	if f, ok := ToFloat64(item); ok {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, NewXPathError("FOCA0002", fmt.Sprintf("cannot cast %v to xs:integer", f))
		}
		return Sequence{int(f)}, nil
	}
	// String-based: must be valid integer lexical form
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	// xs:integer from string: only valid integer lexical form (digits with optional sign)
	// No decimal points, no scientific notation
	if strings.ContainsAny(sv, ".eE") {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:integer", sv))
	}
	if i, err := strconv.ParseInt(sv, 10, 64); err == nil {
		return Sequence{int(i)}, nil
	}
	return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:integer", sv))
}

func xsDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}

	formats := []string{
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, firstarg)
		if err == nil {
			return Sequence{XSDateTime(t)}, nil
		}
	}
	return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:dateTime", firstarg))
}

func xsDuration(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	d, err := ParseXSDuration(firstarg)
	if err != nil {
		return nil, err
	}
	return Sequence{d}, nil
}

func xsBoolean(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	item := args[0][0]
	// For numeric types, use effective boolean value (non-zero = true).
	switch v := item.(type) {
	case float64:
		return Sequence{v != 0 && !math.IsNaN(v)}, nil
	case int:
		return Sequence{v != 0}, nil
	case XSInteger:
		return Sequence{v.V != 0}, nil
	case int64:
		return Sequence{v != 0}, nil
	case bool:
		return Sequence{v}, nil
	}
	// For strings, use XML Schema lexical rules.
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	switch sv {
	case "true", "1":
		return Sequence{true}, nil
	case "false", "0":
		return Sequence{false}, nil
	default:
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:boolean", sv))
	}
}

func xsAnyURI(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	// Validate: reject invalid percent-encoding and bare colons without scheme
	for i := 0; i < len(sv); i++ {
		if sv[i] == '%' {
			if i+2 >= len(sv) {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid xs:anyURI: incomplete percent-encoding in %q", sv))
			}
			h1, h2 := sv[i+1], sv[i+2]
			if !isHexDigit(h1) || !isHexDigit(h2) {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid xs:anyURI: invalid percent-encoding in %q", sv))
			}
		}
	}
	// Reject URIs starting with ":/" (no scheme)
	if len(sv) >= 2 && sv[0] == ':' && sv[1] == '/' {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid xs:anyURI: %q", sv))
	}
	return Sequence{XSAnyURI(sv)}, nil
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// xsUntypedAtomic treats the value as an untyped atomic string.
func xsUntypedAtomic(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{XSUntypedAtomic(sv)}, nil
}

// xsFloat casts to XSFloat.
func xsFloat(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	item := args[0][0]
	if f, ok := ToFloat64(item); ok {
		return Sequence{XSFloat(f)}, nil
	}
	if b, ok := item.(bool); ok {
		if b {
			return Sequence{XSFloat(1)}, nil
		}
		return Sequence{XSFloat(0)}, nil
	}
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	switch sv {
	case "INF":
		return Sequence{XSFloat(math.Inf(1))}, nil
	case "-INF":
		return Sequence{XSFloat(math.Inf(-1))}, nil
	case "NaN":
		return Sequence{XSFloat(math.NaN())}, nil
	}
	upper := strings.ToUpper(sv)
	if strings.Contains(upper, "INF") || upper == "NAN" || upper == "-NAN" || upper == "+NAN" {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:float", sv))
	}
	f, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:float", sv))
	}
	return Sequence{XSFloat(f)}, nil
}

// xsDecimal casts to XSDecimal. Does not allow INF, NaN, or scientific notation.
func xsDecimal(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	item := args[0][0]
	if f, ok := ToFloat64(item); ok {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %v to xs:decimal", f))
		}
		return Sequence{XSDecimal(f)}, nil
	}
	if b, ok := item.(bool); ok {
		if b {
			return Sequence{XSDecimal(1)}, nil
		}
		return Sequence{XSDecimal(0)}, nil
	}
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	sv = strings.TrimSpace(sv)
	// xs:decimal allows only: optional sign, digits, optional decimal point with digits
	// No scientific notation, no INF, no NaN (case-insensitive check)
	upper := strings.ToUpper(sv)
	if strings.Contains(upper, "INF") || strings.Contains(upper, "NAN") {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:decimal", sv))
	}
	if strings.ContainsAny(sv, "eE") {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:decimal", sv))
	}
	f, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:decimal", sv))
	}
	return Sequence{XSDecimal(f)}, nil
}

func init() {
	RegisterFunction(&Function{Name: "time", Namespace: nsXS, F: xsTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "dateTime", Namespace: nsXS, F: xsDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "double", Namespace: nsXS, F: xsDouble, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "float", Namespace: nsXS, F: xsFloat, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "decimal", Namespace: nsXS, F: xsDecimal, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "duration", Namespace: nsXS, F: xsDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "dayTimeDuration", Namespace: nsXS, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) > 0 {
			// If input is already a duration, extract day/time parts
			if d, ok := args[0][0].(XSDuration); ok {
				return Sequence{XSDuration{
					Negative: d.Negative,
					Days:     d.Days, Hours: d.Hours, Minutes: d.Minutes, Seconds: d.Seconds,
				}}, nil
			}
		}
		result, err := xsDuration(ctx, args)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			return result, nil
		}
		d := result[0].(XSDuration)
		// Extract only day/time components (drop years/months)
		return Sequence{XSDuration{
			Negative: d.Negative,
			Days:     d.Days, Hours: d.Hours, Minutes: d.Minutes, Seconds: d.Seconds,
		}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "yearMonthDuration", Namespace: nsXS, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) > 0 {
			// If input is already a duration, extract year/month parts
			if d, ok := args[0][0].(XSDuration); ok {
				return Sequence{XSDuration{
					Negative: d.Negative,
					Years:    d.Years, Months: d.Months,
				}}, nil
			}
		}
		result, err := xsDuration(ctx, args)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			return result, nil
		}
		d := result[0].(XSDuration)
		// Extract only year/month components (drop day/time)
		return Sequence{XSDuration{
			Negative: d.Negative,
			Years:    d.Years, Months: d.Months,
		}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "date", Namespace: nsXS, F: xsDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "integer", Namespace: nsXS, F: xsIntegerTyped(IntInteger), MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "int", Namespace: nsXS, F: xsIntegerTyped(IntInt), MinArg: 1, MaxArg: 1})
	// XSD derived integer types with subtype tags
	intSubtypeMap := map[string]IntSubtype{
		"long": IntLong, "short": IntShort, "byte": IntByte,
		"unsignedLong": IntUnsignedLong, "unsignedInt": IntUnsignedInt,
		"unsignedShort": IntUnsignedShort, "unsignedByte": IntUnsignedByte,
		"nonPositiveInteger": IntNonPositiveInteger, "nonNegativeInteger": IntNonNegativeInteger,
		"negativeInteger": IntNegativeInteger, "positiveInteger": IntPositiveInteger,
	}
	for name, subtype := range intSubtypeMap {
		RegisterFunction(&Function{Name: name, Namespace: nsXS, F: xsIntegerTyped(subtype), MinArg: 1, MaxArg: 1})
	}
	RegisterFunction(&Function{Name: "string", Namespace: nsXS, F: xsStringTyped(StrString), MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: nsXS, F: xsBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "anyURI", Namespace: nsXS, F: xsAnyURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "untypedAtomic", Namespace: nsXS, F: xsUntypedAtomic, MinArg: 1, MaxArg: 1})
	// xs:hexBinary — stores uppercase hex string, validates hex format
	RegisterFunction(&Function{Name: "hexBinary", Namespace: nsXS, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		sv = strings.TrimSpace(sv)
		// Must be even number of hex characters
		if len(sv)%2 != 0 {
			return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:hexBinary: odd length", sv))
		}
		for _, r := range sv {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:hexBinary: invalid hex character", sv))
			}
		}
		return Sequence{XSHexBinary(strings.ToUpper(sv))}, nil
	}, MinArg: 1, MaxArg: 1})
	// xs:base64Binary — validates base64 encoding, converts from hexBinary
	RegisterFunction(&Function{Name: "base64Binary", Namespace: nsXS, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		// Convert from hexBinary: hex → bytes → base64
		if hb, ok := args[0][0].(XSHexBinary); ok {
			hexStr := string(hb)
			bytes, err := hex.DecodeString(hexStr)
			if err != nil {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid hexBinary for base64 conversion: %v", err))
			}
			return Sequence{XSBase64Binary(base64.StdEncoding.EncodeToString(bytes))}, nil
		}
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		sv = strings.TrimSpace(sv)
		// Validate base64: must be groups of 4 base64 chars, with optional = padding
		clean := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				return -1
			}
			return r
		}, sv)
		if len(clean) > 0 {
			if len(clean)%4 != 0 {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid base64Binary: length %d not multiple of 4", len(clean)))
			}
			for i, r := range clean {
				if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' {
					continue
				}
				if r == '=' && i >= len(clean)-2 {
					continue
				}
				return nil, NewXPathError("FORG0001", fmt.Sprintf("invalid base64Binary character at position %d", i))
			}
		}
		return Sequence{XSBase64Binary(sv)}, nil
	}, MinArg: 1, MaxArg: 1})
	// XSD string-derived types with subtype tags
	strSubtypeMap := map[string]StrSubtype{
		"normalizedString": StrNormalizedString, "token": StrToken,
		"language": StrLanguage, "NMTOKEN": StrNMTOKEN, "Name": StrName,
		"NCName": StrNCName, "ID": StrID, "IDREF": StrIDREF, "ENTITY": StrENTITY,
	}
	for name, subtype := range strSubtypeMap {
		RegisterFunction(&Function{Name: name, Namespace: nsXS, F: xsStringTyped(subtype), MinArg: 1, MaxArg: 1})
	}
	// xs:g* calendar types
	RegisterFunction(&Function{Name: "gYear", Namespace: nsXS, F: xsGYear, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "gMonth", Namespace: nsXS, F: xsGMonth, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "gDay", Namespace: nsXS, F: xsGDay, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "gYearMonth", Namespace: nsXS, F: xsGYearMonth, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "gMonthDay", Namespace: nsXS, F: xsGMonthDay, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "QName", Namespace: nsXS, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		var prefix, local, ns string
		if before, after, ok := strings.Cut(sv, ":"); ok {
			prefix = before
			local = after
			if ctx != nil {
				ns = ctx.Namespaces[prefix]
			}
		} else {
			local = sv
		}
		return Sequence{XSQName{Prefix: prefix, Namespace: ns, Localname: local}}, nil
	}, MinArg: 1, MaxArg: 1})
}
