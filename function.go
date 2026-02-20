package goxpath

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/speedata/goxml"
	"golang.org/x/text/unicode/norm"
)

var (
	xpathfunctions   = make(map[string]*Function)
	multipleWSRegexp *regexp.Regexp
)

// XSDate is a date instance
type XSDate time.Time

func (d XSDate) String() string {
	// for example 2004-05-12+01:00
	return time.Time(d).Format("2006-01-02-07:00")
}

// XSDateTime is a date time instance
type XSDateTime time.Time

func (d XSDateTime) String() string {
	// for example 2004-05-12T18:17:15.125Z
	return time.Time(d).Format("2006-01-02T15:04:05.000-07:00")
}

// XSTime is a time instance
type XSTime time.Time

func (d XSTime) String() string {
	// for example 23:17:00.000-05:00
	return time.Time(d).Format("15:04:05.000-07:00")
}

var currentTimeGetter = func() time.Time {
	return time.Now()
}

const (
	nsFN = "http://www.w3.org/2005/xpath-functions"
	nsXS = "http://www.w3.org/2001/XMLSchema"
)

func fnAbs(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := NumberValue(seq)
	return Sequence{math.Abs(itm)}, err
}

func fnAvg(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	sum := 0.0
	for _, itm := range arg {
		n, err := NumberValue(Sequence{itm})
		if err != nil {
			return nil, err
		}
		sum += n
	}
	return Sequence{sum / float64(len(arg))}, nil
}

func fnBoolean(ctx *Context, args []Sequence) (Sequence, error) {
	bv, err := BooleanValue(args[0])
	return Sequence{bv}, err
}

func fnCeiling(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := NumberValue(seq)
	return Sequence{math.Ceil(itm)}, err
}

func fnCodepointEqual(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 || len(args[1]) == 0 {
		return Sequence{}, nil
	}
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{firstarg == secondarg}, nil
}

func fnCodepointsToString(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	var sb strings.Builder
	for _, itm := range inputSeq {
		i, err := ToXSInteger(itm)
		if err != nil {
			return nil, err
		}
		sb.WriteRune(rune(i))
	}
	return Sequence{sb.String()}, nil
}

func fnCompare(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 || len(args[1]) == 0 {
		return Sequence{}, nil
	}
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.Compare(firstarg, secondarg)}, nil
}

func fnConcat(ctx *Context, args []Sequence) (Sequence, error) {
	var str []string

	for _, seq := range args {
		str = append(str, seq.Stringvalue())
	}
	return Sequence{strings.Join(str, "")}, nil
}

func fnContains(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	testSeq := args[1]
	if len(testSeq) == 0 {
		return Sequence{true}, nil
	}

	if len(inputSeq) == 0 {
		// If len inputSeq == 0, return false unless len testSeq == 0 but len
		// testSeq handled above.
		return Sequence{false}, nil
	}
	var err error
	var inputText, testText string
	if inputText, err = StringValue(inputSeq); err != nil {
		return nil, err
	}
	if testText, err = StringValue(testSeq); err != nil {
		return nil, err
	}

	return Sequence{strings.Contains(inputText, testText)}, nil
}

func fnCount(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	return Sequence{len(seq)}, nil
}

func fnCurrentDate(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSDate(currentTimeGetter())}, nil
}

func fnCurrentDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSDateTime(currentTimeGetter())}, nil
}

func fnCurrentTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSTime(currentTimeGetter())}, nil
}

func fnDistinctValues(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	seen := make(map[any]bool)
	result := Sequence{}
	for _, itm := range arg {
		// Convert to comparable value
		var key any
		switch v := itm.(type) {
		case *goxml.Attribute:
			key = v.Value
		case *goxml.Element:
			key, _ = StringValue(Sequence{v})
		default:
			key = itm
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, itm)
		}
	}
	return result, nil
}

func fnEncodeForURI(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{encodeForURI(sv)}, nil
}

// encodeForURI percent-encodes everything except unreserved characters per RFC 3986.
func encodeForURI(s string) string {
	var sb strings.Builder
	for _, b := range []byte(s) {
		if isUnreserved(b) {
			sb.WriteByte(b)
		} else {
			fmt.Fprintf(&sb, "%%%02X", b)
		}
	}
	return sb.String()
}

func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '_' || b == '.' || b == '~'
}

func fnEscapeHTMLURI(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	for _, r := range sv {
		if r >= 0x20 && r <= 0x7E {
			sb.WriteRune(r)
		} else {
			for _, b := range []byte(string(r)) {
				fmt.Fprintf(&sb, "%%%02X", b)
			}
		}
	}
	return Sequence{sb.String()}, nil
}

func fnIRIToURI(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	for _, r := range sv {
		if r <= 0x20 || r >= 0x7F || strings.ContainsRune("<>{}|\\^\"` ", r) {
			for _, b := range []byte(string(r)) {
				fmt.Fprintf(&sb, "%%%02X", b)
			}
		} else {
			sb.WriteRune(r)
		}
	}
	return Sequence{sb.String()}, nil
}

func fnNormalizeUnicode(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	if sv == "" {
		return Sequence{""}, nil
	}
	form := "NFC"
	if len(args) > 1 && len(args[1]) > 0 {
		f, err := StringValue(args[1])
		if err != nil {
			return nil, err
		}
		form = strings.TrimSpace(strings.ToUpper(f))
	}
	if form == "" {
		return Sequence{sv}, nil
	}
	switch form {
	case "NFC":
		return Sequence{norm.NFC.String(sv)}, nil
	case "NFD":
		return Sequence{norm.NFD.String(sv)}, nil
	case "NFKC":
		return Sequence{norm.NFKC.String(sv)}, nil
	case "NFKD":
		return Sequence{norm.NFKD.String(sv)}, nil
	default:
		return nil, fmt.Errorf("FOCH0003: unsupported normalization form %q", form)
	}
}

func fnRoundHalfToEven(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	if math.IsNaN(m) || math.IsInf(m, 0) || m == 0 {
		return Sequence{m}, nil
	}
	precision := 0
	if len(args) > 1 && len(args[1]) > 0 {
		p, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		precision = int(p)
	}
	factor := math.Pow(10, float64(precision))
	scaled := m * factor
	rounded := math.RoundToEven(scaled)
	return Sequence{rounded / factor}, nil
}

func fnData(ctx *Context, args []Sequence) (Sequence, error) {
	var result Sequence
	for _, itm := range args[0] {
		switch t := itm.(type) {
		case *goxml.Element:
			result = append(result, t.Stringvalue())
		case *goxml.Attribute:
			result = append(result, t.Value)
		case goxml.CharData:
			result = append(result, t.Contents)
		case goxml.Comment:
			result = append(result, t.Contents)
		case goxml.ProcInst:
			result = append(result, string(t.Inst))
		case *goxml.ProcInst:
			result = append(result, string(t.Inst))
		default:
			result = append(result, itm)
		}
	}
	return result, nil
}

func getDateTime(args []Sequence) (time.Time, error) {
	if len(args[0]) != 1 {
		return time.Time{}, fmt.Errorf("expected a single dateTime value")
	}
	if dt, ok := args[0][0].(XSDateTime); ok {
		return time.Time(dt), nil
	}
	return time.Time{}, fmt.Errorf("expected xs:dateTime argument")
}

func getDate(args []Sequence) (time.Time, error) {
	if len(args[0]) != 1 {
		return time.Time{}, fmt.Errorf("expected a single date value")
	}
	if d, ok := args[0][0].(XSDate); ok {
		return time.Time(d), nil
	}
	return time.Time{}, fmt.Errorf("expected xs:date argument")
}

func getTime(args []Sequence) (time.Time, error) {
	if len(args[0]) != 1 {
		return time.Time{}, fmt.Errorf("expected a single time value")
	}
	if t, ok := args[0][0].(XSTime); ok {
		return time.Time(t), nil
	}
	return time.Time{}, fmt.Errorf("expected xs:time argument")
}

func timezoneSequence(t time.Time) Sequence {
	_, offset := t.Zone()
	if t.Location() == time.UTC && offset == 0 {
		return Sequence{XSDuration{Hours: 0}}
	}
	if offset == 0 {
		// No explicit timezone
		return Sequence{}
	}
	d := XSDuration{}
	if offset < 0 {
		d.Negative = true
		offset = -offset
	}
	d.Hours = offset / 3600
	d.Minutes = (offset % 3600) / 60
	return Sequence{d}
}

func getDuration(args []Sequence) (XSDuration, error) {
	if len(args[0]) != 1 {
		return XSDuration{}, fmt.Errorf("expected a single duration value")
	}
	if d, ok := args[0][0].(XSDuration); ok {
		return d, nil
	}
	return XSDuration{}, fmt.Errorf("expected xs:duration argument")
}

func signedInt(val int, negative bool) int {
	if negative {
		return -val
	}
	return val
}

func adjustToTimezone(t time.Time, args []Sequence) (time.Time, error) {
	if len(args) < 2 {
		// No timezone argument: adjust to implicit timezone (local)
		return t.In(time.Local), nil
	}
	if len(args[1]) == 0 {
		// Empty sequence: strip timezone (keep local time values)
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.FixedZone("", 0)), nil
	}
	dur, ok := args[1][0].(XSDuration)
	if !ok {
		return time.Time{}, fmt.Errorf("XPTY0004: expected xs:dayTimeDuration for timezone")
	}
	offset := dur.Hours*3600 + dur.Minutes*60
	if dur.Negative {
		offset = -offset
	}
	if offset < -14*3600 || offset > 14*3600 {
		return time.Time{}, fmt.Errorf("FODT0003: timezone offset out of range")
	}
	var loc *time.Location
	if offset == 0 {
		loc = time.UTC
	} else {
		loc = time.FixedZone("", offset)
	}
	return t.In(loc), nil
}

func fnAdjustDateTimeToTimezone(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	dt, ok := args[0][0].(XSDateTime)
	if !ok {
		return nil, fmt.Errorf("XPTY0004: expected xs:dateTime")
	}
	t, err := adjustToTimezone(time.Time(dt), args)
	if err != nil {
		return nil, err
	}
	return Sequence{XSDateTime(t)}, nil
}

func fnAdjustDateToTimezone(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	d, ok := args[0][0].(XSDate)
	if !ok {
		return nil, fmt.Errorf("XPTY0004: expected xs:date")
	}
	t, err := adjustToTimezone(time.Time(d), args)
	if err != nil {
		return nil, err
	}
	return Sequence{XSDate(t)}, nil
}

func fnAdjustTimeToTimezone(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	tv, ok := args[0][0].(XSTime)
	if !ok {
		return nil, fmt.Errorf("XPTY0004: expected xs:time")
	}
	t, err := adjustToTimezone(time.Time(tv), args)
	if err != nil {
		return nil, err
	}
	return Sequence{XSTime(t)}, nil
}

func fnDayFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Day()}, nil
}

func fnDaysFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	return Sequence{signedInt(d.Days, d.Negative)}, nil
}

func fnHoursFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	return Sequence{signedInt(d.Hours, d.Negative)}, nil
}

func fnMinutesFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	return Sequence{signedInt(d.Minutes, d.Negative)}, nil
}

func fnMonthsFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	return Sequence{signedInt(d.Months, d.Negative)}, nil
}

func fnSecondsFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	s := d.Seconds
	if d.Negative {
		s = -s
	}
	return Sequence{s}, nil
}

func fnYearsFromDuration(ctx *Context, args []Sequence) (Sequence, error) {
	d, err := getDuration(args)
	if err != nil {
		return nil, err
	}
	return Sequence{signedInt(d.Years, d.Negative)}, nil
}

func fnDayFromDate(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDate(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Day()}, nil
}

func fnHoursFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Hour()}, nil
}

func fnMinutesFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Minute()}, nil
}

func fnMonthFromDate(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDate(args)
	if err != nil {
		return nil, err
	}
	return Sequence{int(t.Month())}, nil
}

func fnMonthFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{int(t.Month())}, nil
}

func fnSecondsFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{float64(t.Second()) + float64(t.Nanosecond())/1e9}, nil
}

func fnTimezoneFromDate(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDate(args)
	if err != nil {
		return nil, err
	}
	return timezoneSequence(t), nil
}

func fnTimezoneFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return timezoneSequence(t), nil
}

func fnTimezoneFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getTime(args)
	if err != nil {
		return nil, err
	}
	return timezoneSequence(t), nil
}

func fnYearFromDate(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDate(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Year()}, nil
}

func fnYearFromDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	t, err := getDateTime(args)
	if err != nil {
		return nil, err
	}
	return Sequence{t.Year()}, nil
}

func deepEqualItems(a, b Item) bool {
	switch av := a.(type) {
	case *goxml.Element:
		bv, ok := b.(*goxml.Element)
		if !ok {
			return false
		}
		if av.Name != bv.Name || av.Prefix != bv.Prefix {
			return false
		}
		aAttrs := av.Attributes()
		bAttrs := bv.Attributes()
		if len(aAttrs) != len(bAttrs) {
			return false
		}
		for i, aa := range aAttrs {
			if aa.Name != bAttrs[i].Name || aa.Value != bAttrs[i].Value {
				return false
			}
		}
		aChildren := av.Children()
		bChildren := bv.Children()
		if len(aChildren) != len(bChildren) {
			return false
		}
		for i := range aChildren {
			if !deepEqualItems(aChildren[i], bChildren[i]) {
				return false
			}
		}
		return true
	case *goxml.Attribute:
		bv, ok := b.(*goxml.Attribute)
		if !ok {
			return false
		}
		return av.Name == bv.Name && av.Value == bv.Value
	case goxml.CharData:
		bv, ok := b.(goxml.CharData)
		if !ok {
			return false
		}
		return av.Contents == bv.Contents
	case goxml.Comment:
		bv, ok := b.(goxml.Comment)
		if !ok {
			return false
		}
		return av.Contents == bv.Contents
	case goxml.ProcInst:
		bv, ok := b.(goxml.ProcInst)
		if !ok {
			return false
		}
		return av.Target == bv.Target && string(av.Inst) == string(bv.Inst)
	case float64:
		bv, ok := b.(float64)
		if !ok {
			if bi, ok2 := b.(int); ok2 {
				return av == float64(bi)
			}
			return false
		}
		return av == bv
	case int:
		switch bv := b.(type) {
		case int:
			return av == bv
		case float64:
			return float64(av) == bv
		}
		return false
	case string:
		bv, ok := b.(string)
		if !ok {
			return false
		}
		return av == bv
	case bool:
		bv, ok := b.(bool)
		if !ok {
			return false
		}
		return av == bv
	default:
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
}

func fnDeepEqual(ctx *Context, args []Sequence) (Sequence, error) {
	a := args[0]
	b := args[1]
	if len(a) != len(b) {
		return Sequence{false}, nil
	}
	for i := range a {
		if !deepEqualItems(a[i], b[i]) {
			return Sequence{false}, nil
		}
	}
	return Sequence{true}, nil
}

func fnDoc(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	uri, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	if uri == "" {
		return Sequence{}, nil
	}

	// Resolve the URI against the base URI
	resolvedPath := uri
	if !filepath.IsAbs(uri) {
		if ctx.Store != nil {
			if baseURI, ok := ctx.Store["baseURI"].(string); ok && baseURI != "" {
				resolvedPath = filepath.Join(filepath.Dir(baseURI), uri)
			}
		}
	}

	// Initialize Store and doc-cache if needed
	if ctx.Store == nil {
		ctx.Store = make(map[interface{}]interface{})
	}
	cache, ok := ctx.Store["doc-cache"].(map[string]*goxml.XMLDocument)
	if !ok {
		cache = make(map[string]*goxml.XMLDocument)
		ctx.Store["doc-cache"] = cache
	}

	// Check cache
	if doc, ok := cache[resolvedPath]; ok {
		return Sequence{doc}, nil
	}

	// Parse the XML file
	f, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("fn:doc: %w", err)
	}
	defer f.Close()

	doc, err := goxml.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("fn:doc: error parsing %q: %w", resolvedPath, err)
	}

	// Cache the document
	cache[resolvedPath] = doc

	return Sequence{doc}, nil
}

func fnDocAvailable(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{false}, nil
	}
	uri, err := StringValue(args[0])
	if err != nil {
		return Sequence{false}, nil
	}
	if uri == "" {
		return Sequence{false}, nil
	}
	resolvedPath := uri
	if !filepath.IsAbs(uri) {
		if ctx.Store != nil {
			if baseURI, ok := ctx.Store["baseURI"].(string); ok && baseURI != "" {
				resolvedPath = filepath.Join(filepath.Dir(baseURI), uri)
			}
		}
	}
	f, err := os.Open(resolvedPath)
	if err != nil {
		return Sequence{false}, nil
	}
	f.Close()
	return Sequence{true}, nil
}

func fnEmpty(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{len(args[0]) == 0}, nil
}

func fnExactlyOne(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("FORG0005: fn:exactly-one called with a sequence containing %d items", len(args[0]))
	}
	return args[0], nil
}

func fnExists(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{len(args[0]) > 0}, nil
}

func fnEndsWith(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.HasSuffix(firstarg, secondarg)}, nil
}

func fnFalse(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{false}, nil
}

func fnFloor(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	itm, err := NumberValue(seq)
	return Sequence{math.Floor(itm)}, err
}

func fnFormatNumber(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{""}, nil
	}
	num, err := NumberValue(args[0])
	if err != nil {
		return nil, err
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}

	// Parse the picture string
	// Format: [prefix]<integer-part>[.<fraction-part>][suffix]
	// Special characters: # (optional digit), 0 (required digit), . (decimal), , (grouping)

	decimalSep := '.'
	groupingSep := ','
	minusSign := '-'

	// Find decimal point position in picture
	decimalPos := strings.IndexRune(picture, decimalSep)

	var intPart, fracPart string
	if decimalPos >= 0 {
		intPart = picture[:decimalPos]
		fracPart = picture[decimalPos+1:]
	} else {
		intPart = picture
		fracPart = ""
	}

	// Count required digits (0) and optional digits (#) in fraction part
	fracDigits := 0
	for _, r := range fracPart {
		if r == '0' || r == '#' {
			fracDigits++
		}
	}

	// Count grouping in integer part
	groupingSize := 0
	lastGroupPos := strings.LastIndexFunc(intPart, func(r rune) bool { return r == groupingSep })
	if lastGroupPos >= 0 {
		// Count digits after last grouping separator
		for _, r := range intPart[lastGroupPos+1:] {
			if r == '0' || r == '#' {
				groupingSize++
			}
		}
	}

	// Format the number
	isNegative := num < 0
	if isNegative {
		num = -num
	}

	// Round to required fraction digits
	multiplier := math.Pow(10, float64(fracDigits))
	rounded := math.Round(num*multiplier) / multiplier

	// Split into integer and fraction parts
	intVal := int64(rounded)
	fracVal := rounded - float64(intVal)

	// Format integer part
	intStr := fmt.Sprintf("%d", intVal)

	// Add grouping separators
	if groupingSize > 0 {
		var grouped strings.Builder
		for i, r := range intStr {
			if i > 0 && (len(intStr)-i)%groupingSize == 0 {
				grouped.WriteRune(groupingSep)
			}
			grouped.WriteRune(r)
		}
		intStr = grouped.String()
	}

	// Format fraction part
	var result strings.Builder
	if isNegative {
		result.WriteRune(minusSign)
	}
	result.WriteString(intStr)

	if fracDigits > 0 {
		result.WriteRune(decimalSep)
		fracStr := fmt.Sprintf("%.*f", fracDigits, fracVal)
		// Remove "0." prefix from fracStr
		if len(fracStr) > 2 {
			result.WriteString(fracStr[2:])
		} else {
			result.WriteString(strings.Repeat("0", fracDigits))
		}
	}

	return Sequence{result.String()}, nil
}

func fnHoursFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of hours-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of hours-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("15")}, nil
}

func fnIndexOf(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	search := args[1]
	if len(seq) == 0 || len(search) == 0 {
		return Sequence{}, nil
	}

	// Get the search value - must be a single atomic value
	if len(search) != 1 {
		return nil, fmt.Errorf("second argument of index-of must be a single atomic value")
	}
	searchVal := search[0]

	// Convert search value to comparable form
	var searchKey any
	switch v := searchVal.(type) {
	case *goxml.Attribute:
		searchKey = v.Value
	case float64, int, string, bool:
		searchKey = v
	default:
		sv, _ := StringValue(search)
		searchKey = sv
	}

	result := Sequence{}
	for i, itm := range seq {
		var itmKey any
		switch v := itm.(type) {
		case *goxml.Attribute:
			itmKey = v.Value
		case float64, int, string, bool:
			itmKey = v
		default:
			sv, _ := StringValue(Sequence{itm})
			itmKey = sv
		}

		if itmKey == searchKey {
			result = append(result, i+1) // XPath uses 1-based indexing
		}
	}
	return result, nil
}

func fnInScopePrefixes(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("expected a single element for in-scope-prefixes")
	}
	elt, ok := args[0][0].(*goxml.Element)
	if !ok {
		return nil, fmt.Errorf("argument to in-scope-prefixes must be an element")
	}
	var result Sequence
	result = append(result, "xml") // xml prefix is always in scope
	for prefix := range elt.Namespaces {
		if prefix != "" {
			result = append(result, prefix)
		}
	}
	// Default namespace: empty prefix represented as ""
	if _, ok := elt.Namespaces[""]; ok {
		result = append(result, "")
	}
	return result, nil
}

func fnLocalNameFromQName(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("expected a single QName for local-name-from-QName")
	}
	switch q := args[0][0].(type) {
	case XSQName:
		return Sequence{q.Localname}, nil
	default:
		return nil, fmt.Errorf("argument to local-name-from-QName must be an xs:QName")
	}
}

func fnNamespaceURIForPrefix(ctx *Context, args []Sequence) (Sequence, error) {
	prefix, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("second argument to namespace-uri-for-prefix must be a single element")
	}
	elt, ok := args[1][0].(*goxml.Element)
	if !ok {
		return nil, fmt.Errorf("second argument to namespace-uri-for-prefix must be an element")
	}
	if uri, exists := elt.Namespaces[prefix]; exists {
		return Sequence{uri}, nil
	}
	if prefix == "xml" {
		return Sequence{"http://www.w3.org/XML/1998/namespace"}, nil
	}
	return Sequence{}, nil
}

func fnNamespaceURIFromQName(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("expected a single QName for namespace-uri-from-QName")
	}
	switch q := args[0][0].(type) {
	case XSQName:
		return Sequence{q.Namespace}, nil
	default:
		return nil, fmt.Errorf("argument to namespace-uri-from-QName must be an xs:QName")
	}
}

func fnPrefixFromQName(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("expected a single QName for prefix-from-QName")
	}
	switch q := args[0][0].(type) {
	case XSQName:
		if q.Prefix == "" {
			return Sequence{}, nil
		}
		return Sequence{q.Prefix}, nil
	default:
		return nil, fmt.Errorf("argument to prefix-from-QName must be an xs:QName")
	}
}

func fnQName(ctx *Context, args []Sequence) (Sequence, error) {
	uri, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	qnameStr, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	var prefix, localname string
	if idx := strings.IndexByte(qnameStr, ':'); idx >= 0 {
		prefix = qnameStr[:idx]
		localname = qnameStr[idx+1:]
	} else {
		localname = qnameStr
	}
	if prefix != "" && uri == "" {
		return nil, fmt.Errorf("FOCA0002: non-empty prefix %q requires a non-empty namespace URI", prefix)
	}
	return Sequence{XSQName{Namespace: uri, Prefix: prefix, Localname: localname}}, nil
}

func fnResolveQName(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	qnameStr, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("second argument to resolve-QName must be a single element")
	}
	elt, ok := args[1][0].(*goxml.Element)
	if !ok {
		return nil, fmt.Errorf("second argument to resolve-QName must be an element")
	}
	var prefix, localname string
	if idx := strings.IndexByte(qnameStr, ':'); idx >= 0 {
		prefix = qnameStr[:idx]
		localname = qnameStr[idx+1:]
	} else {
		localname = qnameStr
	}
	uri := ""
	if prefix != "" {
		var exists bool
		uri, exists = elt.Namespaces[prefix]
		if !exists {
			return nil, fmt.Errorf("FONS0004: prefix %q not found in in-scope namespaces", prefix)
		}
	} else if defaultNS, exists := elt.Namespaces[""]; exists {
		uri = defaultNS
	}
	return Sequence{XSQName{Namespace: uri, Prefix: prefix, Localname: localname}}, nil
}

func fnResolveURI(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{}, nil
	}
	relative, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	var base string
	if len(args) > 1 && len(args[1]) > 0 {
		base, err = StringValue(args[1])
		if err != nil {
			return nil, err
		}
	} else if ctx.Store != nil {
		if baseURI, ok := ctx.Store["baseURI"].(string); ok {
			base = baseURI
		}
	}
	if base == "" {
		return nil, fmt.Errorf("FORG0002: no base URI available")
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("FORG0002: invalid base URI: %w", err)
	}
	relURL, err := url.Parse(relative)
	if err != nil {
		return nil, fmt.Errorf("FORG0002: invalid relative URI: %w", err)
	}
	return Sequence{baseURL.ResolveReference(relURL).String()}, nil
}

func fnInsertBefore(ctx *Context, args []Sequence) (Sequence, error) {
	target := args[0]
	posVal, err := NumberValue(args[1])
	if err != nil {
		return nil, err
	}
	inserts := args[2]
	pos := int(posVal)
	if pos < 1 {
		pos = 1
	}
	if pos > len(target)+1 {
		pos = len(target) + 1
	}
	result := make(Sequence, 0, len(target)+len(inserts))
	result = append(result, target[:pos-1]...)
	result = append(result, inserts...)
	result = append(result, target[pos-1:]...)
	return result, nil
}

func fnLast(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.size}, nil
}

func fnLang(ctx *Context, args []Sequence) (Sequence, error) {
	testLang, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	var node *goxml.Element
	if len(args) > 1 && len(args[1]) > 0 {
		var ok bool
		if node, ok = args[1][0].(*goxml.Element); !ok {
			return Sequence{false}, nil
		}
	} else if len(ctx.sequence) > 0 {
		var ok bool
		if node, ok = ctx.sequence[0].(*goxml.Element); !ok {
			return Sequence{false}, nil
		}
	} else {
		return Sequence{false}, nil
	}
	testLang = strings.ToLower(testLang)
	for cur := node; cur != nil; {
		for _, attr := range cur.Attributes() {
			if attr.Name == "lang" && attr.Namespace == "http://www.w3.org/XML/1998/namespace" {
				lang := strings.ToLower(attr.Value)
				if lang == testLang || strings.HasPrefix(lang, testLang+"-") {
					return Sequence{true}, nil
				}
				return Sequence{false}, nil
			}
		}
		if parent, ok := cur.Parent.(*goxml.Element); ok {
			cur = parent
		} else {
			break
		}
	}
	return Sequence{false}, nil
}

func fnLocalName(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if elt, ok := arg[0].(*goxml.Element); ok {
		return Sequence{elt.Name}, nil
	}
	if attr, ok := arg[0].(*goxml.Attribute); ok {
		return Sequence{attr.Name}, nil
	}
	return Sequence{""}, nil
}

func fnLowercase(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]

	if len(inputSeq) == 0 {
		return Sequence{""}, nil
	}
	var str string
	var err error
	if str, err = StringValue(inputSeq); err != nil {
		return Sequence{""}, err
	}
	return Sequence{strings.ToLower(str)}, nil
}

func fnMatches(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	regexSeq := args[1]
	if len(inputSeq) == 0 {
		return nil, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}

	if len(regexSeq) == 0 {
		return nil, fmt.Errorf("second argument of fn:matches must be a regular expression")
	}
	regex, err := StringValue(regexSeq)
	if err != nil {
		return nil, err
	}

	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("second argument of fn:matches must be a valid regular expression")
	}

	return Sequence{r.MatchString(input)}, nil
}

func fnMax(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		m = math.Max(m, ai)
	}
	return Sequence{m}, nil
}

func fnMin(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		m = math.Min(m, ai)
	}
	return Sequence{m}, nil
}

func fnMinutesFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of minutes-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of minutes-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("04")}, nil
}

func fnNormalizeSpace(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if len(arg) > 1 {
		return nil, fmt.Errorf("The cardinality of first argument of fn:normalize-string() is zero or one; supplied value has cardinality more than one")
	}
	itm := arg[0]
	if str, ok := itm.(string); ok {
		str = multipleWSRegexp.ReplaceAllString(str, " ")
		str = strings.TrimSpace(str)
		return Sequence{str}, nil
	}
	return Sequence{}, nil
}

func fnName(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if elt, ok := arg[0].(*goxml.Element); ok {
		if pfx := elt.Prefix; pfx != "" {
			return Sequence{pfx + ":" + elt.Name}, nil
		}
		return Sequence{elt.Name}, nil
	}
	if attr, ok := arg[0].(*goxml.Attribute); ok {
		if pfx := attr.Prefix; attr.Prefix != "" {
			return Sequence{pfx + ":" + attr.Name}, nil
		}
		return Sequence{attr.Name}, nil
	}
	return Sequence{""}, nil
}

func fnNamespaceURI(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{""}, nil
	}
	if elt, ok := arg[0].(*goxml.Element); ok {
		return Sequence{elt.Namespaces[elt.Prefix]}, nil
	}
	if attr, ok := arg[0].(*goxml.Attribute); ok {
		return Sequence{attr.Namespace}, nil
	}
	return Sequence{""}, nil
}

func fnNot(ctx *Context, args []Sequence) (Sequence, error) {
	b, err := BooleanValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{!b}, nil
}

func fnNumber(ctx *Context, args []Sequence) (Sequence, error) {
	nv, err := NumberValue(args[0])
	return Sequence{nv}, err
}

func fnOneOrMore(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) < 1 {
		return nil, fmt.Errorf("FORG0004: fn:one-or-more called with a sequence containing zero items")
	}
	return args[0], nil
}

func fnPosition(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.Pos}, nil
}

func fnRemove(ctx *Context, args []Sequence) (Sequence, error) {
	target := args[0]
	posVal, err := NumberValue(args[1])
	if err != nil {
		return nil, err
	}
	pos := int(posVal)
	if pos < 1 || pos > len(target) {
		return target, nil
	}
	result := make(Sequence, 0, len(target)-1)
	result = append(result, target[:pos-1]...)
	result = append(result, target[pos:]...)
	return result, nil
}

func fnReplace(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	regexSeq := args[1]
	replaceSeq := args[2]
	if len(inputSeq) == 0 {
		return nil, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}

	if len(regexSeq) == 0 {
		return nil, fmt.Errorf("second argument of fn:replace must be a regular expression")
	}
	regex, err := StringValue(regexSeq)
	if err != nil {
		return nil, err
	}

	rexpr, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("second argument of fn:replace must be a regular expression")
	}

	replace, err := StringValue(replaceSeq)
	if err != nil {
		return nil, err
	}

	// xpath uses $12 for $12 or $1, depending on the existence of $12 or $1.
	// go on the other hand uses $12 for $12 and never for $1, so you have to write
	// $1 as ${1} if there is text after the $1.
	// We escape the $n backwards to prevent expansion of $12 to ${1}2
	for i := rexpr.NumSubexp(); i > 0; i-- {
		// first create rexepx that match "$i"
		x := fmt.Sprintf(`\$(%d)`, i)
		nummatcher := regexp.MustCompile(x)
		replace = nummatcher.ReplaceAllString(replace, fmt.Sprintf(`$${%d}`, i))
	}
	str := rexpr.ReplaceAllString(input, replace)
	return Sequence{str}, nil
}

func fnReverse(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	if len(inputSeq) == 0 {
		return inputSeq, nil
	}
	retSeq := make(Sequence, len(inputSeq))
	i := 0
	l := len(inputSeq)
	for {
		retSeq[i] = inputSeq[l-i-1]
		i++
		if i >= l {
			break
		}
	}

	return retSeq, nil
}

func fnRoot(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{ctx.Document()}, nil
}

func fnRound(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}

	return Sequence{math.Floor(m + 0.5)}, nil
}

func fnSecondsFromTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg := args[0]
	if len(firstarg) != 1 {
		return nil, fmt.Errorf("The first argument of seconds-from-time must have length(1)")
	}
	var t XSTime
	var ok bool
	if t, ok = firstarg[0].(XSTime); !ok {
		return nil, fmt.Errorf("The argument of seconds-from-time must be xs:time")
	}
	return Sequence{time.Time(t).Format("05")}, nil
}

func fnSum(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{0.0}, nil
	}
	sum := 0.0
	for _, itm := range arg {
		n, err := NumberValue(Sequence{itm})
		if err != nil {
			return nil, err
		}
		sum += n
	}
	return Sequence{sum}, nil
}

func fnString(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	sv, err := StringValue(arg)
	if err != nil {
		return nil, err
	}
	return Sequence{sv}, nil
}

func fnStartsWith(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	return Sequence{strings.HasPrefix(firstarg, secondarg)}, nil
}

func fnStringJoin(ctx *Context, args []Sequence) (Sequence, error) {
	var joiner string
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("Second argument should be a string")
	}
	joiner = itemStringvalue(args[1][0])
	collection := make([]string, len(args[0]))
	for i, itm := range args[0] {
		collection[i] = itemStringvalue(itm)
	}
	return Sequence{strings.Join(collection, joiner)}, nil
}

func fnStringLength(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}

	if len(arg) == 0 {
		return Sequence{0}, nil
	}
	str := itemStringvalue(arg[0])
	// todo: non-string and non-element gives error
	return Sequence{utf8.RuneCountInString(str)}, nil
}

func fnStringToCodepoints(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	if len(inputSeq) == 0 {
		return Sequence{}, nil
	}
	input, err := StringValue(inputSeq)
	if err != nil {
		return nil, err
	}
	var retSeq Sequence

	for _, r := range input {
		retSeq = append(retSeq, int(r))
	}
	return retSeq, nil
}

func fnSubstring(ctx *Context, args []Sequence) (Sequence, error) {
	inputSeq := args[0]
	startSeq := args[1]

	var err error
	var inputText string
	var startNum, lenNum float64
	if inputText, err = StringValue(inputSeq); err != nil {
		return nil, err
	}
	if startNum, err = NumberValue(startSeq); err != nil {
		return nil, err
	}
	inputRunes := []rune(inputText)
	if len(args) > 2 {
		lenSeq := args[2]
		if lenNum, err = NumberValue(lenSeq); err != nil {
			return nil, err
		}
		inputRunes = inputRunes[int(startNum)-1 : int(startNum)+int(lenNum)-1]
		return Sequence{string(inputRunes)}, nil
	}
	return Sequence{string(inputRunes[int(startNum)-1:])}, nil
}

func fnSubstringAfter(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	_, after, _ := strings.Cut(firstarg, secondarg)

	return Sequence{after}, nil
}

func fnSubstringBefore(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	before, _, _ := strings.Cut(firstarg, secondarg)

	return Sequence{before}, nil
}

func fnSubsequence(ctx *Context, args []Sequence) (Sequence, error) {
	sourceSeq := args[0]
	if len(sourceSeq) == 0 {
		return Sequence{}, nil
	}

	startLoc, err := NumberValue(args[1])
	if err != nil {
		return nil, err
	}

	// XPath uses 1-based indexing, convert to 0-based
	// Also handle rounding as per XPath spec: round half to even
	startIdx := int(math.Round(startLoc)) - 1

	var length int
	if len(args) > 2 {
		lengthVal, err := NumberValue(args[2])
		if err != nil {
			return nil, err
		}
		length = int(math.Round(lengthVal))
	} else {
		length = len(sourceSeq) - startIdx
	}

	// Handle negative start index
	if startIdx < 0 {
		length += startIdx
		startIdx = 0
	}

	if startIdx >= len(sourceSeq) || length <= 0 {
		return Sequence{}, nil
	}

	endIdx := min(startIdx+length, len(sourceSeq))

	return sourceSeq[startIdx:endIdx], nil
}

func fnTranslate(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	secondarg, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	thirdarg, err := StringValue(args[2])
	if err != nil {
		return nil, err
	}
	var replace []string
	var i int
	var s rune
	var t string
	thirdArgRunes := []rune(thirdarg)
	for i, s = range secondarg {
		if len(thirdArgRunes) > i {
			t = string(thirdArgRunes[i])
		} else {
			t = ""
		}
		replace = append(replace, string(s), t)
	}
	repl := strings.NewReplacer(replace...)

	return Sequence{repl.Replace(firstarg)}, nil
}

func fnTrue(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{true}, nil
}

func fnUppercase(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]

	if len(arg) == 0 {
		return Sequence{""}, nil
	}

	return Sequence{strings.ToUpper(arg.Stringvalue())}, nil
}

func fnUnordered(ctx *Context, args []Sequence) (Sequence, error) {
	return args[0], nil
}

func fnZeroOrOne(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) > 1 {
		return nil, fmt.Errorf("FORG0003: fn:zero-or-one called with a sequence containing %d items", len(args[0]))
	}
	return args[0], nil
}

func fnTokenize(ctx *Context, args []Sequence) (Sequence, error) {
	input := args[0]
	if len(input) == 0 {
		return Sequence{}, nil
	}
	regexpSeq := args[1]
	if len(regexpSeq) != 1 {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	var regexpStr string
	var ok bool
	if regexpStr, ok = args[1][0].(string); !ok {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	r, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("Second argument of fn:tokenize must be a regular expression")
	}
	text := input.Stringvalue()
	idx := r.FindAllStringIndex(text, -1)

	pos := 0
	var res []string
	for _, v := range idx {
		res = append(res, text[pos:v[0]])
		pos = v[1]
	}
	res = append(res, text[pos:])

	var retSeq Sequence
	for _, str := range res {
		retSeq = append(retSeq, str)
	}
	return retSeq, nil
}

func fnFormatDate(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{""}, nil
	}
	dateVal, ok := args[0][0].(XSDate)
	if !ok {
		return nil, fmt.Errorf("XPTY0004: first argument of format-date must be xs:date, got %T", args[0][0])
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}

	t := time.Time(dateVal)
	var result strings.Builder
	i := 0
	for i < len(picture) {
		if picture[i] == '[' {
			end := strings.IndexByte(picture[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("FOFD1340: unclosed '[' in format-date picture string")
			}
			component := picture[i+1 : i+end]
			i = i + end + 1

			// Trim whitespace from the component
			component = strings.TrimSpace(component)
			if len(component) == 0 {
				continue
			}

			specifier := component[0]
			modifier := component[1:]

			switch specifier {
			case 'Y':
				year := t.Year()
				if modifier == "0001" || modifier == "" {
					result.WriteString(fmt.Sprintf("%04d", year))
				} else {
					result.WriteString(fmt.Sprintf("%d", year))
				}
			case 'M':
				month := int(t.Month())
				if modifier == "01" {
					result.WriteString(fmt.Sprintf("%02d", month))
				} else if modifier == "Nn" || modifier == "n" || modifier == "N" {
					name := t.Month().String()
					if modifier == "n" {
						name = strings.ToLower(name)
					} else if modifier == "N" {
						name = strings.ToUpper(name)
					}
					result.WriteString(name)
				} else {
					result.WriteString(fmt.Sprintf("%d", month))
				}
			case 'D':
				day := t.Day()
				if modifier == "01" {
					result.WriteString(fmt.Sprintf("%02d", day))
				} else {
					result.WriteString(fmt.Sprintf("%d", day))
				}
			default:
				// Unknown component, output as-is
				result.WriteByte('[')
				result.WriteString(component)
				result.WriteByte(']')
			}
		} else {
			result.WriteByte(picture[i])
			i++
		}
	}

	return Sequence{result.String()}, nil
}

func fnSort(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	if len(seq) <= 1 {
		return seq, nil
	}

	// Build sortable slice of (string-value, original-item) pairs
	type sortEntry struct {
		key string
		itm Item
	}
	entries := make([]sortEntry, len(seq))
	for i, itm := range seq {
		sv, err := StringValue(Sequence{itm})
		if err != nil {
			return nil, err
		}
		entries[i] = sortEntry{key: sv, itm: itm}
	}

	// Simple insertion sort (stable)
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].key < entries[j-1].key; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	result := make(Sequence, len(entries))
	for i, e := range entries {
		result[i] = e.itm
	}
	return result, nil
}

func fnCurrentGroupingKey(ctx *Context, args []Sequence) (Sequence, error) {
	if ctx.Store == nil {
		return Sequence{""}, nil
	}
	if key, ok := ctx.Store["current-grouping-key"]; ok {
		if s, ok := key.(string); ok {
			return Sequence{s}, nil
		}
	}
	return Sequence{""}, nil
}

func fnCurrentGroup(ctx *Context, args []Sequence) (Sequence, error) {
	if ctx.Store == nil {
		return Sequence{}, nil
	}
	if group, ok := ctx.Store["current-group"]; ok {
		if seq, ok := group.(Sequence); ok {
			return seq, nil
		}
	}
	return Sequence{}, nil
}

func init() {
	multipleWSRegexp = regexp.MustCompile(`\s+`)
	RegisterFunction(&Function{Name: "abs", Namespace: nsFN, F: fnAbs, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "adjust-date-to-timezone", Namespace: nsFN, F: fnAdjustDateToTimezone, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "adjust-dateTime-to-timezone", Namespace: nsFN, F: fnAdjustDateTimeToTimezone, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "adjust-time-to-timezone", Namespace: nsFN, F: fnAdjustTimeToTimezone, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "avg", Namespace: nsFN, F: fnAvg, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "boolean", Namespace: nsFN, F: fnBoolean, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ceiling", Namespace: nsFN, F: fnCeiling, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "codepoint-equal", Namespace: nsFN, F: fnCodepointEqual, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "codepoints-to-string", Namespace: nsFN, F: fnCodepointsToString, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "compare", Namespace: nsFN, F: fnCompare, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "concat", Namespace: nsFN, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "contains", Namespace: nsFN, F: fnContains, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "count", Namespace: nsFN, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "current-date", Namespace: nsFN, F: fnCurrentDate, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-dateTime", Namespace: nsFN, F: fnCurrentDateTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-time", Namespace: nsFN, F: fnCurrentTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-group", Namespace: nsFN, F: fnCurrentGroup, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-grouping-key", Namespace: nsFN, F: fnCurrentGroupingKey, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "data", Namespace: nsFN, F: fnData, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "deep-equal", Namespace: nsFN, F: fnDeepEqual, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "distinct-values", Namespace: nsFN, F: fnDistinctValues, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "doc", Namespace: nsFN, F: fnDoc, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "doc-available", Namespace: nsFN, F: fnDocAvailable, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "encode-for-uri", Namespace: nsFN, F: fnEncodeForURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "escape-html-uri", Namespace: nsFN, F: fnEscapeHTMLURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "empty", Namespace: nsFN, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "exactly-one", Namespace: nsFN, F: fnExactlyOne, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "exists", Namespace: nsFN, F: fnExists, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ends-with", Namespace: nsFN, F: fnEndsWith, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "false", Namespace: nsFN, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: nsFN, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "format-date", Namespace: nsFN, F: fnFormatDate, MinArg: 2, MaxArg: 5})
	RegisterFunction(&Function{Name: "format-number", Namespace: nsFN, F: fnFormatNumber, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "day-from-date", Namespace: nsFN, F: fnDayFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "day-from-dateTime", Namespace: nsFN, F: fnDayFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "days-from-duration", Namespace: nsFN, F: fnDaysFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-dateTime", Namespace: nsFN, F: fnHoursFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-duration", Namespace: nsFN, F: fnHoursFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-time", Namespace: nsFN, F: fnHoursFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "index-of", Namespace: nsFN, F: fnIndexOf, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "in-scope-prefixes", Namespace: nsFN, F: fnInScopePrefixes, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "insert-before", Namespace: nsFN, F: fnInsertBefore, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "iri-to-uri", Namespace: nsFN, F: fnIRIToURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "lang", Namespace: nsFN, F: fnLang, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "minutes-from-dateTime", Namespace: nsFN, F: fnMinutesFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "minutes-from-duration", Namespace: nsFN, F: fnMinutesFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "minutes-from-time", Namespace: nsFN, F: fnMinutesFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "month-from-date", Namespace: nsFN, F: fnMonthFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "month-from-dateTime", Namespace: nsFN, F: fnMonthFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "months-from-duration", Namespace: nsFN, F: fnMonthsFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "seconds-from-dateTime", Namespace: nsFN, F: fnSecondsFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "seconds-from-duration", Namespace: nsFN, F: fnSecondsFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "seconds-from-time", Namespace: nsFN, F: fnSecondsFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "timezone-from-date", Namespace: nsFN, F: fnTimezoneFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "timezone-from-dateTime", Namespace: nsFN, F: fnTimezoneFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "timezone-from-time", Namespace: nsFN, F: fnTimezoneFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "last", Namespace: nsFN, F: fnLast})
	RegisterFunction(&Function{Name: "local-name", Namespace: nsFN, F: fnLocalName, MaxArg: 1})
	RegisterFunction(&Function{Name: "local-name-from-QName", Namespace: nsFN, F: fnLocalNameFromQName, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "lower-case", Namespace: nsFN, F: fnLowercase, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "matches", Namespace: nsFN, F: fnMatches, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "max", Namespace: nsFN, F: fnMax, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "min", Namespace: nsFN, F: fnMin, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "name", Namespace: nsFN, F: fnName, MaxArg: 1})
	RegisterFunction(&Function{Name: "namespace-uri", Namespace: nsFN, F: fnNamespaceURI, MaxArg: 1})
	RegisterFunction(&Function{Name: "namespace-uri-for-prefix", Namespace: nsFN, F: fnNamespaceURIForPrefix, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "namespace-uri-from-QName", Namespace: nsFN, F: fnNamespaceURIFromQName, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: nsFN, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-space", Namespace: nsFN, F: fnNormalizeSpace, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-unicode", Namespace: nsFN, F: fnNormalizeUnicode, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "number", Namespace: nsFN, F: fnNumber, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "one-or-more", Namespace: nsFN, F: fnOneOrMore, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: nsFN, F: fnPosition})
	RegisterFunction(&Function{Name: "prefix-from-QName", Namespace: nsFN, F: fnPrefixFromQName, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "QName", Namespace: nsFN, F: fnQName, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "remove", Namespace: nsFN, F: fnRemove, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "replace", Namespace: nsFN, F: fnReplace, MinArg: 3, MaxArg: 4})
	RegisterFunction(&Function{Name: "resolve-QName", Namespace: nsFN, F: fnResolveQName, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "resolve-uri", Namespace: nsFN, F: fnResolveURI, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "reverse", Namespace: nsFN, F: fnReverse, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "root", Namespace: nsFN, F: fnRoot, MaxArg: 1})
	RegisterFunction(&Function{Name: "round", Namespace: nsFN, F: fnRound, MaxArg: 1})
	RegisterFunction(&Function{Name: "round-half-to-even", Namespace: nsFN, F: fnRoundHalfToEven, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "sort", Namespace: nsFN, F: fnSort, MinArg: 1, MaxArg: 3})
	RegisterFunction(&Function{Name: "string", Namespace: nsFN, F: fnString, MaxArg: 1})
	RegisterFunction(&Function{Name: "starts-with", Namespace: nsFN, F: fnStartsWith, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-join", Namespace: nsFN, F: fnStringJoin, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-length", Namespace: nsFN, F: fnStringLength, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-to-codepoints", Namespace: nsFN, F: fnStringToCodepoints, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "substring", Namespace: nsFN, F: fnSubstring, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-before", Namespace: nsFN, F: fnSubstringBefore, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-after", Namespace: nsFN, F: fnSubstringAfter, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "subsequence", Namespace: nsFN, F: fnSubsequence, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "sum", Namespace: nsFN, F: fnSum, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "translate", Namespace: nsFN, F: fnTranslate, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "true", Namespace: nsFN, F: fnTrue})
	RegisterFunction(&Function{Name: "tokenize", Namespace: nsFN, F: fnTokenize, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "unordered", Namespace: nsFN, F: fnUnordered, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "upper-case", Namespace: nsFN, F: fnUppercase, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "year-from-date", Namespace: nsFN, F: fnYearFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "year-from-dateTime", Namespace: nsFN, F: fnYearFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "years-from-duration", Namespace: nsFN, F: fnYearsFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "zero-or-one", Namespace: nsFN, F: fnZeroOrOne, MinArg: 1, MaxArg: 1})
}

// Function represents an XPath function
type Function struct {
	Name      string
	Namespace string
	F         func(*Context, []Sequence) (Sequence, error)
	MinArg    int
	MaxArg    int
}

// RegisterFunction registers an XPath function
func RegisterFunction(f *Function) {
	xpathfunctions[f.Namespace+" "+f.Name] = f
}

func getfunction(namespace, name string) *Function {
	return xpathfunctions[namespace+" "+name]
}

func callFunction(name string, arguments []Sequence, ctx *Context) (Sequence, error) {
	var prefix, localName string
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		prefix = name[:idx]
		localName = name[idx+1:]
	} else {
		localName = name
	}
	return callFunctionResolved(prefix, localName, arguments, ctx)
}

func callFunctionResolved(prefix, localName string, arguments []Sequence, ctx *Context) (Sequence, error) {
	var ns string
	if prefix != "" {
		var ok bool
		if ns, ok = ctx.Namespaces[prefix]; !ok {
			return nil, fmt.Errorf("Could not find namespace for prefix %q", prefix)
		}
	} else {
		ns = nsFN
	}

	fn := getfunction(ns, localName)
	if fn == nil {
		return nil, fmt.Errorf("Could not find function %q in namespace %q", localName, ns)
	}
	if min := fn.MinArg; min > 0 {
		if len(arguments) < min {
			return nil, fmt.Errorf("too few arguments in function call (%q), min: %d", fn.Name, fn.MinArg)
		}
	}
	if max := fn.MaxArg; max > -1 {
		if len(arguments) > max {
			return nil, fmt.Errorf("too many arguments in function call (%q), max: %d, got %d (%#v)", fn.Name, fn.MaxArg, len(arguments), arguments)
		}
	}
	return fn.F(ctx, arguments)
}
