package goxpath

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
	nsFN   = "http://www.w3.org/2005/xpath-functions"
	nsXS   = "http://www.w3.org/2001/XMLSchema"
	nsMath = "http://www.w3.org/2005/xpath-functions/math"
)

func fnAbs(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	if len(seq) == 0 {
		return Sequence{}, nil
	}
	itm, err := NumberValue(seq)
	if err != nil {
		return nil, err
	}
	result := math.Abs(itm)
	return Sequence{WrapNumeric(result, NumericType(seq[0]))}, nil
}

// castUntypedToDouble converts all XSUntypedAtomic items in a sequence to XSDouble.
// This implements the fn:min/fn:max/fn:avg spec requirement that xs:untypedAtomic
// values are cast to xs:double. Returns FORG0001 if a value cannot be cast.
func castUntypedToDouble(seq Sequence) (Sequence, error) {
	result := make(Sequence, len(seq))
	for i, itm := range seq {
		if ua, ok := itm.(XSUntypedAtomic); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(string(ua)), 64)
			if err != nil {
				return nil, NewXPathError("FORG0001", fmt.Sprintf("cannot cast %q to xs:double", string(ua)))
			}
			result[i] = XSDouble(f)
		} else {
			result[i] = itm
		}
	}
	return result, nil
}

// atomizeSequence converts each node in the sequence to its typed atomic value.
// Attributes, elements, documents, and text nodes become XSUntypedAtomic strings.
// Non-node items are kept as-is.
func atomizeSequence(seq Sequence) Sequence {
	result := make(Sequence, 0, len(seq))
	for _, itm := range seq {
		switch v := itm.(type) {
		case *goxml.Attribute:
			result = append(result, XSUntypedAtomic(v.Value))
		case *goxml.Element:
			result = append(result, XSUntypedAtomic(v.Stringvalue()))
		case *goxml.XMLDocument:
			result = append(result, XSUntypedAtomic(v.Stringvalue()))
		case goxml.CharData:
			result = append(result, XSUntypedAtomic(v.Contents))
		case *goxml.CharData:
			result = append(result, XSUntypedAtomic(v.Contents))
		default:
			result = append(result, itm)
		}
	}
	return result
}

func fnAvg(ctx *Context, args []Sequence) (Sequence, error) {
	arg := atomizeSequence(args[0])
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	// Cast xs:untypedAtomic to xs:double (raises FORG0001 on failure)
	var err error
	if arg, err = castUntypedToDouble(arg); err != nil {
		return nil, err
	}
	// Validate: all items must be numeric or duration
	for _, itm := range arg {
		if _, ok := ToFloat64(itm); !ok {
			if _, ok := itm.(XSDuration); !ok {
				return nil, NewXPathError("FORG0006", fmt.Sprintf("fn:avg: invalid item type %T", itm))
			}
		}
	}
	// Check for duration avg
	if _, ok := arg[0].(XSDuration); ok {
		ms := durationToMonthsAndSeconds(arg[0].(XSDuration))
		for i := 1; i < len(arg); i++ {
			d, ok := arg[i].(XSDuration)
			if !ok {
				return nil, NewXPathError("FORG0006", "fn:avg: cannot mix durations with other types")
			}
			ms2 := durationToMonthsAndSeconds(d)
			ms.months += ms2.months
			ms.seconds += ms2.seconds
		}
		n := float64(len(arg))
		return Sequence{monthsAndSecondsToDuration(int(float64(ms.months)/n), ms.seconds/n)}, nil
	}
	sum := 0.0
	resultType := NumericType(arg[0])
	for _, itm := range arg {
		n, err := NumberValue(Sequence{itm})
		if err != nil {
			return nil, err
		}
		sum += n
		resultType = PromoteNumeric(resultType, NumericType(itm))
	}
	// avg always returns at least decimal (division)
	if resultType < NumDecimal {
		resultType = NumDecimal
	}
	return Sequence{WrapNumeric(sum/float64(len(arg)), resultType)}, nil
}

func fnBoolean(ctx *Context, args []Sequence) (Sequence, error) {
	bv, err := BooleanValue(args[0])
	return Sequence{bv}, err
}

func fnCeiling(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	if len(seq) == 0 {
		return Sequence{}, nil
	}
	itm, err := NumberValue(seq)
	if err != nil {
		return nil, err
	}
	result := math.Ceil(itm)
	return Sequence{WrapNumeric(result, NumericType(seq[0]))}, nil
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
	return Sequence{XSDate(ctx.CurrentTime())}, nil
}

func fnCurrentDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSDateTime(ctx.CurrentTime())}, nil
}

func fnCurrentTime(ctx *Context, args []Sequence) (Sequence, error) {
	return Sequence{XSTime(ctx.CurrentTime())}, nil
}

func fnDistinctValues(ctx *Context, args []Sequence) (Sequence, error) {
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	seen := make(map[any]bool)
	seenNaN := false
	result := Sequence{}
	for _, itm := range arg {
		// Convert to comparable value, normalizing numeric types so that
		// e.g. XSInteger{134}, XSDecimal(134), and XSDouble(134) are equal.
		var key any
		switch v := itm.(type) {
		case *goxml.Attribute:
			key = v.Value
		case *goxml.Element:
			key, _ = StringValue(Sequence{v})
		default:
			if f, ok := ToFloat64(itm); ok {
				if math.IsNaN(f) {
					if seenNaN {
						continue
					}
					seenNaN = true
					result = append(result, itm)
					continue
				}
				key = f
			} else {
				key = itm
			}
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
		return nil, NewXPathError("FOCH0003", fmt.Sprintf("unsupported normalization form %q", form))
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
	nt := NumericType(arg[0])
	if math.IsNaN(m) || math.IsInf(m, 0) || m == 0 {
		return Sequence{WrapNumeric(m, nt)}, nil
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
	return Sequence{WrapNumeric(rounded/factor, nt)}, nil
}

func fnData(ctx *Context, args []Sequence) (Sequence, error) {
	var input Sequence
	if len(args) == 0 {
		input = ctx.sequence
	} else {
		input = args[0]
	}
	var result Sequence
	for _, itm := range input {
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
		return time.Time{}, NewXPathError("XPTY0004", "expected xs:dayTimeDuration for timezone")
	}
	offset := dur.Hours*3600 + dur.Minutes*60
	if dur.Negative {
		offset = -offset
	}
	if offset < -14*3600 || offset > 14*3600 {
		return time.Time{}, NewXPathError("FODT0003", "timezone offset out of range")
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
		return nil, NewXPathError("XPTY0004", "expected xs:dateTime")
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
		return nil, NewXPathError("XPTY0004", "expected xs:date")
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
		return nil, NewXPathError("XPTY0004", "expected xs:time")
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
		// Attribute order is not significant for deep-equal; match by name.
		for _, aa := range aAttrs {
			found := false
			for _, ba := range bAttrs {
				if aa.Name == ba.Name && aa.Namespace == ba.Namespace {
					if aa.Value != ba.Value {
						return false
					}
					found = true
					break
				}
			}
			if !found {
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
		bv, ok := ToFloat64(b)
		if !ok {
			return false
		}
		return av == bv
	case int:
		bv, ok := ToFloat64(b)
		if !ok {
			return false
		}
		return float64(av) == bv
	case XSInteger:
		bv, ok := ToFloat64(b)
		if !ok {
			return false
		}
		return float64(av.V) == bv
	case XSString:
		if bv, ok := b.(XSString); ok {
			return av.V == bv.V
		}
		if bv, ok := b.(string); ok {
			return av.V == bv
		}
		return false
	case string:
		if bv, ok := b.(XSString); ok {
			return av == bv.V
		}
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
		ctx.Store = make(map[any]any)
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
		return nil, NewXPathError("FORG0005", fmt.Sprintf("fn:exactly-one called with a sequence containing %d items", len(args[0])))
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
	if len(seq) == 0 {
		return Sequence{}, nil
	}
	itm, err := NumberValue(seq)
	if err != nil {
		return nil, err
	}
	result := math.Floor(itm)
	return Sequence{WrapNumeric(result, NumericType(seq[0]))}, nil
}

func fnFormatNumber(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		// Look up format for NaN string
		fmtName := ""
		if len(args) >= 3 && len(args[2]) > 0 {
			fmtName, _ = StringValue(args[2])
		}
		emptyDf, _ := ctx.GetDecimalFormat(fmtName)
		if emptyDf == nil {
			emptyDf = DefaultDecimalFormat()
		}
		return Sequence{emptyDf.NaN}, nil
	}
	num, err := NumberValue(args[0])
	if err != nil {
		return nil, err
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}

	// Look up decimal format
	formatName := ""
	if len(args) >= 3 && len(args[2]) > 0 {
		formatName, _ = StringValue(args[2])
	}
	df, err := ctx.GetDecimalFormat(formatName)
	if err != nil {
		return nil, err
	}

	decimalSep := df.DecimalSeparator
	groupingSep := df.GroupingSeparator
	minusSign := df.MinusSign
	percent := df.Percent
	perMille := df.PerMille
	zeroDigit := df.ZeroDigit
	digit := df.Digit
	patternSep := df.PatternSeparator

	// Split into positive and negative sub-pictures
	posPic := picture
	negPic := ""
	if idx := strings.IndexRune(picture, patternSep); idx >= 0 {
		posPic = picture[:idx]
		negPic = picture[idx+1:]
	}

	// Validate picture string (FODF1310)
	if err := validateFormatNumberPicture(posPic, decimalSep, groupingSep); err != nil {
		return nil, err
	}
	if negPic != "" {
		if err := validateFormatNumberPicture(negPic, decimalSep, groupingSep); err != nil {
			return nil, err
		}
	}

	// Handle special values (NaN, +Infinity, -Infinity)
	// Per spec: output is prefix + special-string + suffix from the active sub-picture
	if math.IsNaN(num) {
		prefix, suffix := extractPicturePrefixSuffix(posPic, decimalSep, groupingSep, digit, zeroDigit)
		return Sequence{prefix + df.NaN + suffix}, nil
	}
	isNegative := math.IsInf(num, -1) || (num < 0 && !math.IsNaN(num)) || math.Signbit(num)
	if math.IsInf(num, 1) {
		prefix, suffix := extractPicturePrefixSuffix(posPic, decimalSep, groupingSep, digit, zeroDigit)
		return Sequence{prefix + df.Infinity + suffix}, nil
	}
	if math.IsInf(num, -1) {
		activePic := posPic
		if negPic != "" {
			activePic = negPic
		}
		prefix, suffix := extractPicturePrefixSuffix(activePic, decimalSep, groupingSep, digit, zeroDigit)
		if negPic == "" {
			prefix = string(minusSign) + prefix
		}
		return Sequence{prefix + df.Infinity + suffix}, nil
	}

	if isNegative {
		num = -num
	}

	// Check for percent/per-mille
	multiplier := ""
	activePic := posPic
	if isNegative && negPic != "" {
		activePic = negPic
	}
	if strings.ContainsRune(activePic, percent) {
		num *= 100
		multiplier = string(percent)
		// Check for overflow to infinity after percent multiplication
		if math.IsInf(num, 0) {
			prefix, suffix := extractPicturePrefixSuffix(activePic, decimalSep, groupingSep, digit, zeroDigit)
			if isNegative {
				prefix = string(minusSign) + prefix
			}
			return Sequence{prefix + df.Infinity + multiplier + suffix}, nil
		}
	} else if strings.ContainsRune(activePic, perMille) {
		num *= 1000
		multiplier = string(perMille)
		if math.IsInf(num, 0) {
			prefix, suffix := extractPicturePrefixSuffix(activePic, decimalSep, groupingSep, digit, zeroDigit)
			if isNegative {
				prefix = string(minusSign) + prefix
			}
			return Sequence{prefix + df.Infinity + multiplier + suffix}, nil
		}
	}

	prefix := ""
	if isNegative && negPic == "" {
		prefix = string(minusSign)
	}

	result := formatSubPicture(num, activePic, decimalSep, groupingSep, zeroDigit, digit, multiplier)
	return Sequence{prefix + result}, nil
}

// detectDigitFamily detects the Unicode decimal digit family of a rune.
// Returns the zero digit of the family and true, or 0 and false if not a digit.
func detectDigitFamily(r rune) (rune, bool) {
	// Check standard Unicode decimal digit ranges
	// Each range has 10 consecutive code points (0-9)
	digitRanges := [][2]rune{
		{0x0030, 0x0039},   // ASCII 0-9
		{0x0660, 0x0669},   // Arabic-Indic ٠-٩
		{0x06F0, 0x06F9},   // Extended Arabic-Indic ۰-۹
		{0x0966, 0x096F},   // Devanagari ०-९
		{0x09E6, 0x09EF},   // Bengali ০-৯
		{0x0A66, 0x0A6F},   // Gurmukhi ੦-੯
		{0x0AE6, 0x0AEF},   // Gujarati ૦-૯
		{0x0B66, 0x0B6F},   // Oriya ୦-୯
		{0x0BE6, 0x0BEF},   // Tamil ௦-௯
		{0x0C66, 0x0C6F},   // Telugu ౦-౯
		{0x0CE6, 0x0CEF},   // Kannada ೦-೯
		{0x0D66, 0x0D6F},   // Malayalam ൦-൯
		{0x0E50, 0x0E59},   // Thai ๐-๙
		{0x0ED0, 0x0ED9},   // Lao ໐-໙
		{0x0F20, 0x0F29},   // Tibetan ༠-༩
		{0x1040, 0x1049},   // Myanmar ၀-၉
		{0x17E0, 0x17E9},   // Khmer ០-៩
		{0x1810, 0x1819},   // Mongolian ᠐-᠙
		{0xFF10, 0xFF19},   // Fullwidth ０-９
		{0x104A0, 0x104A9}, // Osmanya 𐒠-𐒩
	}
	for _, rng := range digitRanges {
		if r >= rng[0] && r <= rng[1] {
			return rng[0], true
		}
	}
	return 0, false
}

// translateDigits replaces ASCII digits 0-9 in s with the corresponding
// digits from the family whose zero digit is zeroRune.
func translateDigits(s string, zeroRune rune) string {
	if zeroRune == '0' {
		return s // ASCII, no translation needed
	}
	var result []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = append(result, zeroRune+r-'0')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// extractPicturePrefixSuffix extracts the prefix and suffix from a sub-picture.
// The prefix is everything before the first digit/grouping/decimal character,
// the suffix is everything after the last one.
func extractPicturePrefixSuffix(pic string, decSep, grpSep, dig, zero rune) (string, string) {
	runes := []rune(pic)
	isDigitChar := func(i int) bool {
		r := runes[i]
		if r == zero || r == dig || r == decSep || r == grpSep || (r >= '0' && r <= '9') {
			return true
		}
		// 'e' is only a digit char if followed by a digit
		if r == 'e' && i+1 < len(runes) {
			next := runes[i+1]
			if next == zero || next == dig || (next >= '0' && next <= '9') {
				return true
			}
			if _, ok := detectDigitFamily(next); ok {
				return true
			}
		}
		_, ok := detectDigitFamily(r)
		return ok
	}
	start, end := 0, len(runes)
	for start < len(runes) && !isDigitChar(start) {
		start++
	}
	for end > start && !isDigitChar(end-1) {
		end--
	}
	prefix := string(runes[:start])
	suffix := string(runes[end:])
	// Remove percent/per-mille from prefix/suffix (they're format characters, not literals)
	prefix = strings.ReplaceAll(prefix, "%", "")
	prefix = strings.ReplaceAll(prefix, "\u2030", "")
	suffix = strings.ReplaceAll(suffix, "%", "")
	suffix = strings.ReplaceAll(suffix, "\u2030", "")
	return prefix, suffix
}

// validateFormatNumberPicture checks a format-number sub-picture for FODF1310 errors.
func validateFormatNumberPicture(pic string, decSep, grpSep rune) error {
	runes := []rune(pic)
	// Find the digit pattern area
	isDigitChar := func(r rune) bool {
		if r == '0' || r == '#' || r == decSep || r == grpSep || r == 'e' || (r >= '1' && r <= '9') {
			return true
		}
		_, ok := detectDigitFamily(r)
		return ok
	}
	start, end := 0, len(runes)
	for start < len(runes) && !isDigitChar(runes[start]) {
		start++
	}
	for end > start && !isDigitChar(runes[end-1]) {
		end--
	}
	if start >= end {
		return nil // no digit pattern, e.g. pure prefix/suffix (valid for special values)
	}
	pattern := runes[start:end]

	// Check for uppercase E (invalid as exponent separator)
	if slices.Contains(pattern, 'E') {
		return NewXPathError("FODF1310", "invalid picture: 'E' is not a valid exponent separator (use 'e')")
	}

	// Check for multiple exponent separators ('e' followed by digit)
	eCount := 0
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == 'e' {
			nextIsDigit := pattern[i+1] == '0' || pattern[i+1] == '#' || (pattern[i+1] >= '1' && pattern[i+1] <= '9')
			if !nextIsDigit {
				_, nextIsDigit = detectDigitFamily(pattern[i+1])
			}
			if nextIsDigit {
				eCount++
			}
		}
	}
	if eCount > 1 {
		return NewXPathError("FODF1310", "invalid picture: multiple exponent separators")
	}

	// Check for exponent + percent/per-mille
	hasExp := false
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == 'e' {
			isExpDigit := pattern[i+1] == '0' || pattern[i+1] == '#' || (pattern[i+1] >= '1' && pattern[i+1] <= '9')
			if !isExpDigit {
				_, isExpDigit = detectDigitFamily(pattern[i+1])
			}
			if isExpDigit {
				hasExp = true
			}
		}
	}
	if hasExp {
		for _, r := range runes {
			if r == '%' || r == '\u2030' {
				return NewXPathError("FODF1310", "invalid picture: exponent with percent/per-mille")
			}
		}
	}

	// Note: 'e' in prefix/suffix is not an exponent separator.
	// Only check 'e' that has a mandatory digit character before it.
	// The hasExp check above already correctly identifies exponent 'e'.

	// Check for adjacent grouping separators or grouping at edges
	for i := range pattern {
		if pattern[i] == grpSep {
			// Trailing grouping separator
			if i == len(pattern)-1 {
				return NewXPathError("FODF1310", "invalid picture: trailing grouping separator")
			}
			// Adjacent grouping separators
			if i+1 < len(pattern) && pattern[i+1] == grpSep {
				return NewXPathError("FODF1310", "invalid picture: adjacent grouping separators")
			}
			// Grouping separator adjacent to decimal separator
			if i+1 < len(pattern) && pattern[i+1] == decSep {
				return NewXPathError("FODF1310", "invalid picture: grouping separator adjacent to decimal separator")
			}
			if i > 0 && pattern[i-1] == decSep {
				return NewXPathError("FODF1310", "invalid picture: grouping separator adjacent to decimal separator")
			}
		}
	}

	// Check for no digit before exponent (e.g., ".e99")
	if hasExp {
		hasDigitBeforeExp := false
		// Find the actual exponent 'e' position (the one followed by a digit)
		for i := 0; i < len(pattern)-1; i++ {
			r := pattern[i]
			if r == '0' || r == '#' || r == decSep || r == grpSep {
				hasDigitBeforeExp = hasDigitBeforeExp || r == '0' || r == '#'
			}
			if _, ok := detectDigitFamily(r); ok {
				hasDigitBeforeExp = true
			}
			if r == 'e' {
				nextIsDigit := pattern[i+1] == '0' || pattern[i+1] == '#' || (pattern[i+1] >= '1' && pattern[i+1] <= '9')
				if !nextIsDigit {
					_, nextIsDigit = detectDigitFamily(pattern[i+1])
				}
				if nextIsDigit {
					// This is the actual exponent separator
					if !hasDigitBeforeExp {
						return NewXPathError("FODF1310", "invalid picture: no digit before exponent separator")
					}
					break
				}
			}
		}
	}

	// Check for '0' after '#' in the fraction part (invalid)
	decIdx := -1
	for i, r := range pattern {
		if r == decSep {
			decIdx = i
			break
		}
	}
	if decIdx >= 0 {
		seenOptional := false
		for i := decIdx + 1; i < len(pattern); i++ {
			r := pattern[i]
			// Stop at exponent separator
			if r == 'e' {
				break
			}
			if r == grpSep {
				continue
			}
			if r == '#' {
				seenOptional = true
			} else if r == '0' && seenOptional {
				return NewXPathError("FODF1310", "invalid picture: '0' after '#' in fraction part")
			}
		}
	}

	return nil
}

// formatSubPicture formats a non-negative number according to a sub-picture.
func formatSubPicture(num float64, pic string, decSep, grpSep, zero, dig rune, multiplier string) string {
	// Detect non-ASCII digit family in the picture
	outputZero := zero // the zero digit for output
	runes := []rune(pic)
	for _, r := range runes {
		if z, ok := detectDigitFamily(r); ok && z != '0' {
			outputZero = z
			break
		}
	}

	// Extract prefix and suffix (non-digit characters before/after the digit pattern)
	isDigitChar := func(r rune) bool {
		if r == zero || r == dig || r == decSep || r == grpSep {
			return true
		}
		// Also match non-ASCII digits from the detected family
		if _, ok := detectDigitFamily(r); ok {
			return true
		}
		return false
	}

	// Find start and end of digit pattern
	start, end := 0, len(runes)
	for start < len(runes) && !isDigitChar(runes[start]) {
		start++
	}
	for end > start && !isDigitChar(runes[end-1]) {
		end--
	}
	prefix := string(runes[:start])
	suffix := string(runes[end:])

	// Remove multiplier characters from prefix/suffix (they're applied separately)
	if multiplier != "" {
		prefix = strings.ReplaceAll(prefix, multiplier, "")
		suffix = strings.ReplaceAll(suffix, multiplier, "")
	}

	pattern := runes[start:end]

	// Split pattern at decimal separator
	decPos := -1
	for i, r := range pattern {
		if r == decSep {
			decPos = i
			break
		}
	}

	// Check for exponent sub-picture: look for 'e' followed by digits in the pattern
	expPos := -1
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == 'e' {
			nextIsDigit := pattern[i+1] == zero || pattern[i+1] == dig || (pattern[i+1] >= '0' && pattern[i+1] <= '9')
			if !nextIsDigit {
				_, nextIsDigit = detectDigitFamily(pattern[i+1])
			}
			if nextIsDigit {
				expPos = i
				break
			}
		}
	}

	var expPattern []rune
	mainPattern := pattern
	if expPos >= 0 {
		mainPattern = pattern[:expPos]
		expPattern = pattern[expPos+1:]
	}

	var intPattern, fracPattern []rune
	if decPos >= 0 && (expPos < 0 || decPos < expPos) {
		intPattern = mainPattern[:decPos]
		fracPattern = mainPattern[decPos+1:]
	} else {
		intPattern = mainPattern
	}

	// Analyze integer pattern: count min digits (0), collect grouping positions
	minIntDigits := 0
	var intGroupPositions []int // positions from right where separators go
	digitCount := 0
	for i := len(intPattern) - 1; i >= 0; i-- {
		r := intPattern[i]
		isDigit := r == zero
		if !isDigit {
			if _, ok := detectDigitFamily(r); ok && r != dig {
				isDigit = true
			}
		}
		if isDigit {
			minIntDigits++
			digitCount++
		} else if intPattern[i] == dig {
			digitCount++
		} else if intPattern[i] == grpSep {
			intGroupPositions = append(intGroupPositions, digitCount)
		}
	}
	if minIntDigits == 0 && decPos < 0 {
		minIntDigits = 1 // at least one digit if no fraction
	}

	// Analyze fraction pattern
	minFracDigits := 0
	maxFracDigits := 0
	for _, r := range fracPattern {
		isFracZero := r == zero
		if !isFracZero {
			if _, ok := detectDigitFamily(r); ok && r != dig {
				isFracZero = true
			}
		}
		if isFracZero {
			minFracDigits++
			maxFracDigits++
		} else if r == dig {
			maxFracDigits++
		}
	}

	// Handle exponent format
	var exponent int
	hasExponent := len(expPattern) > 0
	if hasExponent {
		// Count exponent digits
		minExpDigits := 0
		for _, r := range expPattern {
			if r == zero || (r >= '0' && r <= '9') {
				minExpDigits++
			}
		}
		if minExpDigits == 0 {
			minExpDigits = 1
		}

		// Normalize: the number of integer digits in the mantissa equals
		// the number of mandatory digits (0) in the integer part of the pattern.
		// E.g., "999.99e99" → 3 integer digits, "#99.99e99" → 2, "#.99e99" → 0
		mantissaIntDigits := minIntDigits

		if num != 0 {
			logVal := math.Floor(math.Log10(num))
			if mantissaIntDigits == 0 {
				// No mandatory integer digits: mantissa is 0.xxx
				exponent = int(logVal) + 1
			} else {
				exponent = int(logVal) - (mantissaIntDigits - 1)
			}
			num = num / math.Pow(10, float64(exponent))
		}
	}

	// Round number to maxFracDigits
	factor := math.Pow(10, float64(maxFracDigits))
	rounded := math.Round(num*factor) / factor

	// Split into integer and fraction
	intVal := int64(rounded)
	fracVal := rounded - float64(intVal)
	if fracVal < 0 {
		fracVal = 0
	}

	// Format integer part
	intStr := fmt.Sprintf("%d", intVal)
	// Pad to minimum digits
	for len(intStr) < minIntDigits {
		intStr = "0" + intStr
	}
	// Remove leading zeros if pattern uses # (minIntDigits == 0)
	if minIntDigits == 0 && intVal == 0 {
		if maxFracDigits > 0 {
			if hasExponent && digitCount > 0 {
				// Exponent with # in integer part: keep "0" (e.g., "#.#e0" → "0.2e0")
				intStr = "0"
			} else {
				// Non-exponent or no integer digit slot: suppress (e.g., "#.#" → ".0")
				intStr = ""
			}
		}
		// If no fraction digits at all, keep at least "0"
		if maxFracDigits == 0 && minFracDigits == 0 {
			intStr = "0"
		}
	}

	// Add grouping separators with support for irregular grouping
	// intGroupPositions contains cumulative positions from right: [2, 5] means
	// separator after 2 digits, then after 5 digits from right.
	// Group sizes are the differences: [2, 3]. The last (leftmost) repeats.
	if len(intGroupPositions) > 0 {
		// Convert cumulative positions to group sizes
		groupSizes := make([]int, len(intGroupPositions))
		groupSizes[0] = intGroupPositions[0]
		for i := 1; i < len(intGroupPositions); i++ {
			groupSizes[i] = intGroupPositions[i] - intGroupPositions[i-1]
		}

		// Determine grouping mode:
		// - Single separator or all equal spacing: regular, repeats
		// - Multiple separators, unequal spacing: only at defined positions
		isRepeating := true
		for i := 1; i < len(groupSizes); i++ {
			if groupSizes[i] != groupSizes[0] {
				isRepeating = false
				break
			}
		}

		var grouped []rune
		intRunes := []rune(intStr)
		digitsSinceGroup := 0
		groupIdx := 0
		currentGroupSize := groupSizes[0]
		for i := len(intRunes) - 1; i >= 0; i-- {
			if digitsSinceGroup > 0 && digitsSinceGroup == currentGroupSize {
				grouped = append([]rune{grpSep}, grouped...)
				digitsSinceGroup = 0
				if groupIdx+1 < len(groupSizes) {
					groupIdx++
					currentGroupSize = groupSizes[groupIdx]
				} else if isRepeating {
					// Regular multi-separator: repeat the group size
				} else {
					// Single separator or irregular: no more separators
					currentGroupSize = len(intRunes) + 1
				}
			}
			grouped = append([]rune{intRunes[i]}, grouped...)
			digitsSinceGroup++
		}
		intStr = string(grouped)
	}

	// Format fraction part
	var fracStr string
	if maxFracDigits > 0 {
		fracRaw := fmt.Sprintf("%.*f", maxFracDigits, fracVal)
		// Remove "0." prefix
		if len(fracRaw) > 2 {
			fracRaw = fracRaw[2:]
		} else {
			fracRaw = strings.Repeat("0", maxFracDigits)
		}
		// Trim trailing zeros beyond minFracDigits
		fracRunes := []rune(fracRaw)
		trimTo := len(fracRunes)
		for trimTo > minFracDigits && fracRunes[trimTo-1] == '0' {
			trimTo--
		}
		fracStr = string(fracRunes[:trimTo])
	}

	// Ensure at least one digit when everything would be empty
	if intStr == "" && fracStr == "" && maxFracDigits > 0 {
		fracStr = "0"
	}

	// Build result
	var result strings.Builder
	result.WriteString(prefix)
	result.WriteString(intStr)
	if len(fracStr) > 0 || minFracDigits > 0 {
		result.WriteRune(decSep)
		result.WriteString(fracStr)
	}
	// Add exponent if pattern has one
	if hasExponent {
		result.WriteRune('e')
		minExpDigits := 0
		for _, r := range expPattern {
			if r == zero || (r >= '0' && r <= '9') {
				minExpDigits++
			}
		}
		if minExpDigits == 0 {
			minExpDigits = 1
		}
		if exponent < 0 {
			result.WriteRune('-')
			fmt.Fprintf(&result, "%0*d", minExpDigits, -exponent)
		} else {
			fmt.Fprintf(&result, "%0*d", minExpDigits, exponent)
		}
	}
	if multiplier != "" {
		result.WriteString(multiplier)
	}
	result.WriteString(suffix)

	// Translate digits to the detected digit family
	if outputZero != '0' {
		return translateDigits(result.String(), outputZero)
	}
	return result.String()
}

// numberToWords converts an integer to English words.
func numberToWords(n int) string {
	if n == 0 {
		return "zero"
	}
	neg := ""
	if n < 0 {
		neg = "minus "
		n = -n
	}

	ones := []string{
		"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen",
	}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

	var parts []string
	if n >= 1000000000 {
		parts = append(parts, numberToWords(n/1000000000)+" billion")
		n %= 1000000000
	}
	if n >= 1000000 {
		parts = append(parts, numberToWords(n/1000000)+" million")
		n %= 1000000
	}
	if n >= 1000 {
		parts = append(parts, numberToWords(n/1000)+" thousand")
		n %= 1000
	}
	if n >= 100 {
		parts = append(parts, ones[n/100]+" hundred")
		n %= 100
		if n > 0 {
			parts = append(parts, "and")
		}
	}
	if n >= 20 {
		w := tens[n/10]
		if n%10 > 0 {
			w += "-" + ones[n%10]
		}
		parts = append(parts, w)
	} else if n > 0 {
		parts = append(parts, ones[n])
	}

	return neg + strings.Join(parts, " ")
}

// ordinalSuffix returns the English ordinal suffix for n ("st", "nd", "rd", "th").
func ordinalSuffix(n int) string {
	if n < 0 {
		n = -n
	}
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// numberToOrdinalWords converts an integer to English ordinal words.
func numberToOrdinalWords(n int) string {
	w := numberToWords(n)
	// Convert last word to ordinal form
	irregulars := map[string]string{
		"one": "first", "two": "second", "three": "third", "four": "fourth",
		"five": "fifth", "six": "sixth", "seven": "seventh", "eight": "eighth",
		"nine": "ninth", "ten": "tenth", "eleven": "eleventh", "twelve": "twelfth",
	}
	// Find the last word
	parts := strings.Split(w, " ")
	last := parts[len(parts)-1]
	// Handle hyphenated like "twenty-one"
	if idx := strings.LastIndex(last, "-"); idx >= 0 {
		suffix := last[idx+1:]
		if ord, ok := irregulars[suffix]; ok {
			parts[len(parts)-1] = last[:idx+1] + ord
			return strings.Join(parts, " ")
		}
	}
	if ord, ok := irregulars[last]; ok {
		parts[len(parts)-1] = ord
		return strings.Join(parts, " ")
	}
	// Regular: strip trailing "y" → "ieth", else append "th"
	if strings.HasSuffix(last, "y") {
		parts[len(parts)-1] = last[:len(last)-1] + "ieth"
	} else {
		parts[len(parts)-1] = last + "th"
	}
	return strings.Join(parts, " ")
}

func fnFormatInteger(ctx *Context, args []Sequence) (Sequence, error) {
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
	intVal := int(num)

	// Split off ordinal modifier: "1;o", "Ww;o(-er)", etc.
	mainPic := picture
	ordinalMod := ""
	if before, after, ok := strings.Cut(picture, ";"); ok {
		mainPic = before
		ordinalMod = after
	}

	switch mainPic {
	case "A":
		return Sequence{formatIntegerAlpha(intVal, 'A')}, nil
	case "a":
		return Sequence{formatIntegerAlpha(intVal, 'a')}, nil
	case "I":
		return Sequence{formatRoman(intVal, true)}, nil
	case "i":
		return Sequence{formatRoman(intVal, false)}, nil
	case "W":
		w := numberToWords(intVal)
		if strings.HasPrefix(ordinalMod, "o") {
			w = numberToOrdinalWords(intVal)
		}
		return Sequence{strings.ToUpper(w)}, nil
	case "w":
		w := numberToWords(intVal)
		if strings.HasPrefix(ordinalMod, "o") {
			w = numberToOrdinalWords(intVal)
		}
		return Sequence{w}, nil
	case "Ww":
		w := numberToWords(intVal)
		if strings.HasPrefix(ordinalMod, "o") {
			w = numberToOrdinalWords(intVal)
		}
		// Title case: capitalize first letter of each word
		words := strings.Fields(w)
		for i, word := range words {
			if len(word) > 0 && word != "and" {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
		return Sequence{strings.Join(words, " ")}, nil
	default:
		// Handle ordinal modifier on numeric pictures
		if strings.HasPrefix(ordinalMod, "o") {
			// Detect digit family from main picture
			outZero := '0'
			for _, r := range mainPic {
				if z, ok := detectDigitFamily(r); ok {
					outZero = z
					break
				}
			}
			_ = outZero

			// Format the number then append ordinal suffix
			s := fmt.Sprintf("%d", intVal)
			s += ordinalSuffix(intVal)
			return Sequence{s}, nil
		}
		// Detect digit family from picture
		outZero := '0'
		for _, r := range picture {
			if z, ok := detectDigitFamily(r); ok {
				outZero = z
				break
			}
		}

		// Count digits and detect grouping
		picRunes := []rune(picture)
		minDigits := 0
		var groupPositions []int
		digitPos := 0
		grpChar := rune(0)
		// Scan right to left for grouping
		for i := len(picRunes) - 1; i >= 0; i-- {
			r := picRunes[i]
			if r == '0' || r == '#' {
				if r == '0' {
					minDigits++
				}
				digitPos++
			} else if _, ok := detectDigitFamily(r); ok {
				minDigits++
				digitPos++
			} else if r != '#' && digitPos > 0 {
				// Grouping separator
				grpChar = r
				groupPositions = append(groupPositions, digitPos)
			}
		}
		if minDigits == 0 {
			minDigits = 1
		}

		// Format the number
		isNeg := intVal < 0
		if isNeg {
			intVal = -intVal
		}
		s := fmt.Sprintf("%0*d", minDigits, intVal)

		// Apply grouping
		if len(groupPositions) > 0 && grpChar != 0 {
			groupSizes := make([]int, len(groupPositions))
			groupSizes[0] = groupPositions[0]
			for i := 1; i < len(groupPositions); i++ {
				groupSizes[i] = groupPositions[i] - groupPositions[i-1]
			}
			var grouped []rune
			sRunes := []rune(s)
			dsg := 0
			gi := 0
			cgs := groupSizes[0]
			for i := len(sRunes) - 1; i >= 0; i-- {
				if dsg > 0 && dsg == cgs {
					grouped = append([]rune{grpChar}, grouped...)
					dsg = 0
					if gi+1 < len(groupSizes) {
						gi++
						cgs = groupSizes[gi]
					}
				}
				grouped = append([]rune{sRunes[i]}, grouped...)
				dsg++
			}
			s = string(grouped)
		}

		// Translate digits
		if outZero != '0' {
			s = translateDigits(s, outZero)
		}

		if isNeg {
			s = "-" + s
		}
		return Sequence{s}, nil
	}
}

func formatIntegerAlpha(n int, base rune) string {
	if n <= 0 {
		return fmt.Sprintf("%d", n)
	}
	var result []rune
	for n > 0 {
		n--
		result = append([]rune{rune(int(base) + n%26)}, result...)
		n /= 26
	}
	return string(result)
}

func formatRoman(n int, upper bool) string {
	if n <= 0 || n >= 4000 {
		return fmt.Sprintf("%d", n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var sb strings.Builder
	for i, v := range vals {
		for n >= v {
			sb.WriteString(syms[i])
			n -= v
		}
	}
	if upper {
		return sb.String()
	}
	return strings.ToLower(sb.String())
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
	if before, after, ok := strings.Cut(qnameStr, ":"); ok {
		prefix = before
		localname = after
	} else {
		localname = qnameStr
	}
	if prefix != "" && uri == "" {
		return nil, NewXPathError("FOCA0002", fmt.Sprintf("non-empty prefix %q requires a non-empty namespace URI", prefix))
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
	if before, after, ok0 := strings.Cut(qnameStr, ":"); ok0 {
		prefix = before
		localname = after
	} else {
		localname = qnameStr
	}
	uri := ""
	if prefix != "" {
		var exists bool
		uri, exists = elt.Namespaces[prefix]
		if !exists {
			return nil, NewXPathError("FONS0004", fmt.Sprintf("prefix %q not found in in-scope namespaces", prefix))
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
		return nil, NewXPathError("FORG0002", "no base URI available")
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, NewXPathError("FORG0002", fmt.Sprintf("invalid base URI: %v", err))
	}
	relURL, err := url.Parse(relative)
	if err != nil {
		return nil, NewXPathError("FORG0002", fmt.Sprintf("invalid relative URI: %v", err))
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
	pos := min(max(int(posVal), 1), len(target)+1)
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

// xpathRegexToGo translates XPath regular expression syntax to Go regexp syntax.
// Handles XPath-specific features: \i, \c, \I, \C character classes,
// \p{IsBlockName} Unicode block references, and character class subtraction.
func xpathRegexToGo(pattern string) string {
	// Pre-process: remove character class subtraction [X-[Y]] → [X]
	// This is a simplification — it doesn't actually subtract, but prevents parse errors.
	pattern = removeCharClassSubtraction(pattern)

	// Map of XPath Unicode block names to Go character ranges (Unicode 3.1 blocks)
	blockMap := map[string]string{
		"IsBasicLatin":                           `\x{0000}-\x{007F}`,
		"IsLatin-1Supplement":                    `\x{0080}-\x{00FF}`,
		"IsLatinExtended-A":                      `\x{0100}-\x{017F}`,
		"IsLatinExtended-B":                      `\x{0180}-\x{024F}`,
		"IsIPAExtensions":                        `\x{0250}-\x{02AF}`,
		"IsSpacingModifierLetters":               `\x{02B0}-\x{02FF}`,
		"IsCombiningDiacriticalMarks":            `\x{0300}-\x{036F}`,
		"IsGreek":                                `\x{0370}-\x{03FF}`,
		"IsGreekandCoptic":                       `\x{0370}-\x{03FF}`,
		"IsCyrillic":                             `\x{0400}-\x{04FF}`,
		"IsArmenian":                             `\x{0530}-\x{058F}`,
		"IsHebrew":                               `\x{0590}-\x{05FF}`,
		"IsArabic":                               `\x{0600}-\x{06FF}`,
		"IsSyriac":                               `\x{0700}-\x{074F}`,
		"IsThaana":                               `\x{0780}-\x{07BF}`,
		"IsDevanagari":                           `\x{0900}-\x{097F}`,
		"IsBengali":                              `\x{0980}-\x{09FF}`,
		"IsGurmukhi":                             `\x{0A00}-\x{0A7F}`,
		"IsGujarati":                             `\x{0A80}-\x{0AFF}`,
		"IsOriya":                                `\x{0B00}-\x{0B7F}`,
		"IsTamil":                                `\x{0B80}-\x{0BFF}`,
		"IsTelugu":                               `\x{0C00}-\x{0C7F}`,
		"IsKannada":                              `\x{0C80}-\x{0CFF}`,
		"IsMalayalam":                            `\x{0D00}-\x{0D7F}`,
		"IsSinhala":                              `\x{0D80}-\x{0DFF}`,
		"IsThai":                                 `\x{0E00}-\x{0E7F}`,
		"IsLao":                                  `\x{0E80}-\x{0EFF}`,
		"IsTibetan":                              `\x{0F00}-\x{0FFF}`,
		"IsMyanmar":                              `\x{1000}-\x{109F}`,
		"IsGeorgian":                             `\x{10A0}-\x{10FF}`,
		"IsHangulJamo":                           `\x{1100}-\x{11FF}`,
		"IsEthiopic":                             `\x{1200}-\x{137F}`,
		"IsCherokee":                             `\x{13A0}-\x{13FF}`,
		"IsUnifiedCanadianAboriginalSyllabics":   `\x{1400}-\x{167F}`,
		"IsOgham":                                `\x{1680}-\x{169F}`,
		"IsRunic":                                `\x{16A0}-\x{16FF}`,
		"IsKhmer":                                `\x{1780}-\x{17FF}`,
		"IsMongolian":                            `\x{1800}-\x{18AF}`,
		"IsLatinExtendedAdditional":              `\x{1E00}-\x{1EFF}`,
		"IsGreekExtended":                        `\x{1F00}-\x{1FFF}`,
		"IsGeneralPunctuation":                   `\x{2000}-\x{206F}`,
		"IsSuperscriptsandSubscripts":            `\x{2070}-\x{209F}`,
		"IsCurrencySymbols":                      `\x{20A0}-\x{20CF}`,
		"IsCombiningDiacriticalMarksforSymbols":  `\x{20D0}-\x{20FF}`,
		"IsLetterlikeSymbols":                    `\x{2100}-\x{214F}`,
		"IsNumberForms":                          `\x{2150}-\x{218F}`,
		"IsArrows":                               `\x{2190}-\x{21FF}`,
		"IsMathematicalOperators":                `\x{2200}-\x{22FF}`,
		"IsMiscellaneousTechnical":               `\x{2300}-\x{23FF}`,
		"IsControlPictures":                      `\x{2400}-\x{243F}`,
		"IsOpticalCharacterRecognition":          `\x{2440}-\x{245F}`,
		"IsEnclosedAlphanumerics":                `\x{2460}-\x{24FF}`,
		"IsBoxDrawing":                           `\x{2500}-\x{257F}`,
		"IsBlockElements":                        `\x{2580}-\x{259F}`,
		"IsGeometricShapes":                      `\x{25A0}-\x{25FF}`,
		"IsMiscellaneousSymbols":                 `\x{2600}-\x{26FF}`,
		"IsDingbats":                             `\x{2700}-\x{27BF}`,
		"IsBraillePatterns":                      `\x{2800}-\x{28FF}`,
		"IsCJKRadicalsSupplement":                `\x{2E80}-\x{2EFF}`,
		"IsKangxiRadicals":                       `\x{2F00}-\x{2FDF}`,
		"IsIdeographicDescriptionCharacters":     `\x{2FF0}-\x{2FFF}`,
		"IsCJKSymbolsandPunctuation":             `\x{3000}-\x{303F}`,
		"IsHiragana":                             `\x{3040}-\x{309F}`,
		"IsKatakana":                             `\x{30A0}-\x{30FF}`,
		"IsBopomofo":                             `\x{3100}-\x{312F}`,
		"IsHangulCompatibilityJamo":              `\x{3130}-\x{318F}`,
		"IsKanbun":                               `\x{3190}-\x{319F}`,
		"IsBopomofoExtended":                     `\x{31A0}-\x{31BF}`,
		"IsEnclosedCJKLettersandMonths":          `\x{3200}-\x{32FF}`,
		"IsCJKCompatibility":                     `\x{3300}-\x{33FF}`,
		"IsCJKUnifiedIdeographsExtensionA":       `\x{3400}-\x{4DBF}`,
		"IsCJKUnifiedIdeographs":                 `\x{4E00}-\x{9FFF}`,
		"IsYiSyllables":                          `\x{A000}-\x{A48F}`,
		"IsYiRadicals":                           `\x{A490}-\x{A4CF}`,
		"IsHangulSyllables":                      `\x{AC00}-\x{D7AF}`,
		"IsPrivateUse":                           `\x{E000}-\x{F8FF}`,
		"IsPrivateUseArea":                       `\x{E000}-\x{F8FF}`,
		"IsCJKCompatibilityIdeographs":           `\x{F900}-\x{FAFF}`,
		"IsAlphabeticPresentationForms":          `\x{FB00}-\x{FB4F}`,
		"IsArabicPresentationForms-A":            `\x{FB50}-\x{FDFF}`,
		"IsCombiningHalfMarks":                   `\x{FE20}-\x{FE2F}`,
		"IsCJKCompatibilityForms":                `\x{FE30}-\x{FE4F}`,
		"IsSmallFormVariants":                    `\x{FE50}-\x{FE6F}`,
		"IsArabicPresentationForms-B":            `\x{FE70}-\x{FEFF}`,
		"IsHalfwidthandFullwidthForms":           `\x{FF00}-\x{FFEF}`,
		"IsSpecials":                             `\x{FFF0}-\x{FFFF}`,
		"IsOldItalic":                            `\x{10300}-\x{1032F}`,
		"IsGothic":                               `\x{10330}-\x{1034F}`,
		"IsDeseret":                              `\x{10400}-\x{1044F}`,
		"IsByzantineMusicalSymbols":              `\x{1D000}-\x{1D0FF}`,
		"IsMusicalSymbols":                       `\x{1D100}-\x{1D1FF}`,
		"IsMathematicalAlphanumericSymbols":      `\x{1D400}-\x{1D7FF}`,
		"IsCJKUnifiedIdeographsExtensionB":       `\x{20000}-\x{2A6DF}`,
		"IsCJKCompatibilityIdeographsSupplement": `\x{2F800}-\x{2FA1F}`,
		"IsTags":                                 `\x{E0000}-\x{E007F}`,
		"IsSupplementaryPrivateUseArea-A":        `\x{F0000}-\x{FFFFF}`,
		"IsSupplementaryPrivateUseArea-B":        `\x{100000}-\x{10FFFF}`,
	}

	var result strings.Builder
	runes := []rune(pattern)
	i := 0
	for i < len(runes) {
		if runes[i] == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			switch next {
			case 'i':
				// XML Name start character (simplified)
				result.WriteString(`[a-zA-Z_\x{C0}-\x{D6}\x{D8}-\x{F6}\x{F8}-\x{2FF}\x{370}-\x{37D}\x{37F}-\x{1FFF}\x{200C}-\x{200D}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}]`)
				i += 2
				continue
			case 'I':
				result.WriteString(`[^a-zA-Z_\x{C0}-\x{D6}\x{D8}-\x{F6}\x{F8}-\x{2FF}\x{370}-\x{37D}\x{37F}-\x{1FFF}\x{200C}-\x{200D}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}]`)
				i += 2
				continue
			case 'c':
				// XML Name character (simplified: NameStartChar + digits, -, ., combining chars)
				result.WriteString(`[a-zA-Z0-9_\-.\x{B7}\x{C0}-\x{D6}\x{D8}-\x{F6}\x{F8}-\x{2FF}\x{300}-\x{36F}\x{370}-\x{37D}\x{37F}-\x{1FFF}\x{200C}-\x{200D}\x{203F}-\x{2040}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}]`)
				i += 2
				continue
			case 'C':
				result.WriteString(`[^a-zA-Z0-9_\-.\x{B7}\x{C0}-\x{D6}\x{D8}-\x{F6}\x{F8}-\x{2FF}\x{300}-\x{36F}\x{370}-\x{37D}\x{37F}-\x{1FFF}\x{200C}-\x{200D}\x{203F}-\x{2040}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}]`)
				i += 2
				continue
			case 'p', 'P':
				// Unicode property/block: \p{Name} or \P{Name}
				if i+2 < len(runes) && runes[i+2] == '{' {
					end := i + 3
					for end < len(runes) && runes[end] != '}' {
						end++
					}
					if end < len(runes) {
						propName := string(runes[i+3 : end])
						if blockRange, ok := blockMap[propName]; ok {
							if next == 'p' {
								result.WriteString("[" + blockRange + "]")
							} else {
								result.WriteString("[^" + blockRange + "]")
							}
							i = end + 1
							continue
						}
					}
				}
			}
		}
		result.WriteRune(runes[i])
		i++
	}
	return result.String()
}

// removeCharClassSubtraction handles XPath character class subtraction syntax.
// [a-z-[aeiou]] → [a-z] (simplified: removes the subtraction part)
func removeCharClassSubtraction(pattern string) string {
	runes := []rune(pattern)
	var result []rune
	i := 0
	for i < len(runes) {
		if runes[i] == '[' {
			// Find the matching ] accounting for nested [ ]
			result = append(result, runes[i])
			i++
			depth := 1
			for i < len(runes) && depth > 0 {
				if runes[i] == '-' && i+1 < len(runes) && runes[i+1] == '[' {
					// Character class subtraction: skip -[...]
					i += 2 // skip -[
					subDepth := 1
					for i < len(runes) && subDepth > 0 {
						switch runes[i] {
						case '[':
							subDepth++
						case ']':
							subDepth--
						}
						i++
					}
					continue
				}
				switch runes[i] {
				case '[':
					depth++
				case ']':
					depth--
				}
				result = append(result, runes[i])
				i++
			}
		} else {
			result = append(result, runes[i])
			i++
		}
	}
	return string(result)
}

// compileXPathRegex compiles an XPath regular expression, translating XPath-specific
// syntax to Go regexp syntax first.
func compileXPathRegex(pattern string, flags string) (*regexp.Regexp, error) {
	goPattern := xpathRegexToGo(pattern)

	// Handle XPath flags
	prefix := ""
	for _, f := range flags {
		switch f {
		case 's':
			prefix += "(?s)"
		case 'm':
			prefix += "(?m)"
		case 'i':
			prefix += "(?i)"
		case 'x':
			// Free-spacing: remove unescaped whitespace and #-comments
			var cleaned strings.Builder
			runes := []rune(goPattern)
			for j := 0; j < len(runes); j++ {
				if runes[j] == '\\' && j+1 < len(runes) {
					cleaned.WriteRune(runes[j])
					j++
					cleaned.WriteRune(runes[j])
				} else if runes[j] == '#' {
					// Skip to end of line
					for j < len(runes) && runes[j] != '\n' {
						j++
					}
				} else if runes[j] != ' ' && runes[j] != '\t' && runes[j] != '\n' && runes[j] != '\r' {
					cleaned.WriteRune(runes[j])
				}
			}
			goPattern = cleaned.String()
		}
	}

	// XPath-specific regex validation before compilation
	if err := validateXPathRegex(goPattern); err != nil {
		return nil, err
	}

	return regexp.Compile(prefix + goPattern)
}

// validateXPathRegex checks for XPath-specific regex syntax errors that
// Go's RE2 would silently accept.
func validateXPathRegex(pattern string) error {
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			// \x is not valid in XPath regex (must be \x{...})
			if next == 'x' && (i+2 >= len(runes) || runes[i+2] != '{') {
				return fmt.Errorf("invalid escape \\x (use \\x{...} for hex)")
			}
			i++ // skip next
			continue
		}
		if runes[i] == '{' {
			// Check for invalid quantifier: {,n} or unclosed {
			j := i + 1
			// Read until } or end
			for j < len(runes) && runes[j] != '}' {
				j++
			}
			if j >= len(runes) {
				return fmt.Errorf("unclosed '{' in regex")
			}
			content := string(runes[i+1 : j])
			// {,n} without minimum is invalid in XPath
			if len(content) > 0 && content[0] == ',' {
				return fmt.Errorf("invalid quantifier {%s}", content)
			}
		}
	}
	return nil
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

	var flags string
	if len(args) >= 3 {
		flags, _ = StringValue(args[2])
	}
	r, err := compileXPathRegex(regex, flags)
	if err != nil {
		return nil, NewXPathError("FORX0002", fmt.Sprintf("invalid regular expression: %v", err))
	}

	return Sequence{r.MatchString(input)}, nil
}

func fnMax(ctx *Context, args []Sequence) (Sequence, error) {
	arg := atomizeSequence(args[0])
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	// Cast xs:untypedAtomic to xs:double (raises FORG0001 on failure)
	var err error
	if arg, err = castUntypedToDouble(arg); err != nil {
		return nil, err
	}
	if err := validateComparableSequence(arg); err != nil {
		return nil, err
	}
	if isStringLike(arg[0]) {
		m := itemStringvalue(arg[0])
		for i := 1; i < len(arg); i++ {
			s := itemStringvalue(arg[i])
			if s > m {
				m = s
			}
		}
		return Sequence{m}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	resultType := NumericType(arg[0])
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		resultType = PromoteNumeric(resultType, NumericType(arg[i]))
		m = math.Max(m, ai)
	}
	return Sequence{WrapNumeric(m, resultType)}, nil
}

func fnMin(ctx *Context, args []Sequence) (Sequence, error) {
	arg := atomizeSequence(args[0])
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	// Cast xs:untypedAtomic to xs:double (raises FORG0001 on failure)
	var err error
	if arg, err = castUntypedToDouble(arg); err != nil {
		return nil, err
	}
	// Validate: reject incomparable types (QName, mixed string/numeric)
	if err := validateComparableSequence(arg); err != nil {
		return nil, err
	}
	// If first item is a string, compare all items as strings.
	if isStringLike(arg[0]) {
		m := itemStringvalue(arg[0])
		for i := 1; i < len(arg); i++ {
			s := itemStringvalue(arg[i])
			if s < m {
				m = s
			}
		}
		return Sequence{m}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}
	resultType := NumericType(arg[0])
	for i := 1; i < len(arg); i++ {
		ai, err := NumberValue(Sequence{arg[i]})
		if err != nil {
			return nil, err
		}
		resultType = PromoteNumeric(resultType, NumericType(arg[i]))
		m = math.Min(m, ai)
	}
	return Sequence{WrapNumeric(m, resultType)}, nil
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
	str := itemStringvalue(arg[0])
	str = multipleWSRegexp.ReplaceAllString(str, " ")
	str = strings.TrimSpace(str)
	return Sequence{str}, nil
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

func fnNodeName(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	if elt, ok := arg[0].(*goxml.Element); ok {
		ns := elt.Namespaces[elt.Prefix]
		return Sequence{XSQName{Namespace: ns, Prefix: elt.Prefix, Localname: elt.Name}}, nil
	}
	if attr, ok := arg[0].(*goxml.Attribute); ok {
		ns := ""
		if attr.Prefix != "" {
			if parent, ok := attr.Parent.(*goxml.Element); ok {
				ns = parent.Namespaces[attr.Prefix]
			}
		}
		return Sequence{XSQName{Namespace: ns, Prefix: attr.Prefix, Localname: attr.Name}}, nil
	}
	if pi, ok := arg[0].(goxml.ProcInst); ok {
		return Sequence{XSQName{Localname: pi.Target}}, nil
	}
	return Sequence{}, nil
}

func fnNilled(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	// Without schema awareness, nilled() always returns false for elements.
	if _, ok := arg[0].(*goxml.Element); ok {
		return Sequence{false}, nil
	}
	return Sequence{}, nil
}

func fnHasChildren(ctx *Context, args []Sequence) (Sequence, error) {
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{false}, nil
	}
	if node, ok := arg[0].(goxml.XMLNode); ok {
		return Sequence{len(node.Children()) > 0}, nil
	}
	return Sequence{false}, nil
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
	var arg Sequence
	if len(args) == 0 {
		arg = ctx.sequence
	} else {
		arg = args[0]
	}
	if len(arg) == 0 {
		return Sequence{math.NaN()}, nil
	}
	// Handle booleans directly
	if b, ok := arg[0].(bool); ok {
		if b {
			return Sequence{1.0}, nil
		}
		return Sequence{0.0}, nil
	}
	nv, err := NumberValue(arg)
	return Sequence{nv}, err
}

func fnOneOrMore(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) < 1 {
		return nil, NewXPathError("FORG0004", "fn:one-or-more called with a sequence containing zero items")
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

	var flags string
	if len(args) >= 4 {
		flags, _ = StringValue(args[3])
	}
	rexpr, err := compileXPathRegex(regex, flags)
	if err != nil {
		return nil, NewXPathError("FORX0002", fmt.Sprintf("invalid regular expression: %v", err))
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
	if len(args) == 0 {
		return nil, fmt.Errorf("fn:round requires at least one argument")
	}
	arg := args[0]
	if len(arg) == 0 {
		return Sequence{}, nil
	}
	m, err := NumberValue(Sequence{arg[0]})
	if err != nil {
		return nil, err
	}

	nt := NumericType(arg[0])

	if math.IsNaN(m) || math.IsInf(m, 0) || m == 0 {
		return Sequence{WrapNumeric(m, nt)}, nil
	}

	// 2-argument form: round($arg, $precision)
	precision := 0
	if len(args) > 1 && len(args[1]) > 0 {
		p, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		precision = int(p)
	}

	if precision == 0 {
		r := math.Floor(m + 0.5)
		// Preserve negative zero per XPath spec
		if r == 0 && math.Signbit(m) {
			r = math.Copysign(0, -1)
		}
		return Sequence{WrapNumeric(r, nt)}, nil
	}

	factor := math.Pow(10, float64(precision))
	r := math.Floor(m*factor+0.5) / factor
	return Sequence{WrapNumeric(r, nt)}, nil
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
		if len(args) >= 2 {
			return args[1], nil
		}
		return Sequence{0}, nil
	}
	// Check for duration sum
	if _, ok := arg[0].(XSDuration); ok {
		ms := durationToMonthsAndSeconds(arg[0].(XSDuration))
		for i := 1; i < len(arg); i++ {
			d, ok := arg[i].(XSDuration)
			if !ok {
				return nil, NewXPathError("FORG0006", "fn:sum: cannot mix durations with other types")
			}
			ms2 := durationToMonthsAndSeconds(d)
			ms.months += ms2.months
			ms.seconds += ms2.seconds
		}
		return Sequence{monthsAndSecondsToDuration(ms.months, ms.seconds)}, nil
	}
	sum := 0.0
	resultType := NumericType(arg[0])
	for _, itm := range arg {
		n, err := NumberValue(Sequence{itm})
		if err != nil {
			return nil, err
		}
		sum += n
		resultType = PromoteNumeric(resultType, NumericType(itm))
	}
	return Sequence{WrapNumeric(sum, resultType)}, nil
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
	if len(args) >= 2 {
		if len(args[1]) != 1 {
			return nil, fmt.Errorf("Second argument should be a string")
		}
		joiner = itemStringvalue(args[1][0])
	}
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
	var startNum float64
	if inputText, err = StringValue(inputSeq); err != nil {
		return nil, err
	}
	if startNum, err = NumberValue(startSeq); err != nil {
		return nil, err
	}
	inputRunes := []rune(inputText)
	runeLen := len(inputRunes)

	// XPath spec: positions are 1-based, arguments are rounded,
	// and the result is clipped to the actual string bounds.
	start := int(math.Round(startNum)) - 1 // convert to 0-based
	if len(args) > 2 {
		var lenNum float64
		if lenNum, err = NumberValue(args[2]); err != nil {
			return nil, err
		}
		end := start + int(math.Round(lenNum))
		if start < 0 {
			start = 0
		}
		if end > runeLen {
			end = runeLen
		}
		if start >= end {
			return Sequence{""}, nil
		}
		return Sequence{string(inputRunes[start:end])}, nil
	}
	if start < 0 {
		start = 0
	}
	if start >= runeLen {
		return Sequence{""}, nil
	}
	return Sequence{string(inputRunes[start:])}, nil
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
	before, _, found := strings.Cut(firstarg, secondarg)
	if !found {
		return Sequence{""}, nil
	}
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

	// Handle NaN/Inf
	if math.IsNaN(startLoc) {
		return Sequence{}, nil
	}
	if math.IsInf(startLoc, 1) {
		return Sequence{}, nil
	}

	// XPath uses 1-based indexing, convert to 0-based
	// Also handle rounding as per XPath spec: round half to even
	startRounded := math.Round(startLoc)

	var lengthRounded float64
	if len(args) > 2 {
		lengthVal, err := NumberValue(args[2])
		if err != nil {
			return nil, err
		}
		if math.IsNaN(lengthVal) {
			return Sequence{}, nil
		}
		lengthRounded = math.Round(lengthVal)
	} else {
		lengthRounded = float64(len(sourceSeq)) - startRounded + 2
	}

	// Compute effective start and end using float arithmetic (avoids overflow)
	endFloat := startRounded + lengthRounded
	startFloat := startRounded

	// Clamp to valid range
	if startFloat < 1 {
		startFloat = 1
	}
	if endFloat > float64(len(sourceSeq))+1 {
		endFloat = float64(len(sourceSeq)) + 1
	}

	startIdx := int(startFloat) - 1
	endIdx := int(endFloat) - 1

	if startIdx >= endIdx || startIdx >= len(sourceSeq) {
		return Sequence{}, nil
	}

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
		return nil, NewXPathError("FORG0003", fmt.Sprintf("fn:zero-or-one called with a sequence containing %d items", len(args[0])))
	}
	return args[0], nil
}

func fnTokenize(ctx *Context, args []Sequence) (Sequence, error) {
	input := args[0]
	if len(input) == 0 {
		return Sequence{}, nil
	}

	// XPath 3.1: tokenize with 1 argument splits by whitespace
	if len(args) == 1 {
		text := input.Stringvalue()
		text = strings.TrimSpace(text)
		if text == "" {
			return Sequence{}, nil
		}
		parts := strings.Fields(text)
		var retSeq Sequence
		for _, p := range parts {
			retSeq = append(retSeq, p)
		}
		return retSeq, nil
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
	var flags string
	if len(args) >= 3 {
		flags, _ = StringValue(args[2])
		// Validate flags: only s, m, i, x are allowed
		for _, f := range flags {
			if f != 's' && f != 'm' && f != 'i' && f != 'x' {
				return nil, NewXPathError("FORX0001", fmt.Sprintf("invalid flag '%c' in fn:tokenize", f))
			}
		}
	}
	r, err := compileXPathRegex(regexpStr, flags)
	if err != nil {
		return nil, NewXPathError("FORX0002", fmt.Sprintf("invalid regular expression: %v", err))
	}
	text := input.Stringvalue()

	// FORX0003: pattern must not match empty string
	if r.MatchString("") {
		return nil, NewXPathError("FORX0003", "pattern matches empty string in fn:tokenize")
	}

	// Empty input → empty sequence
	if text == "" {
		return Sequence{}, nil
	}

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
		return nil, NewXPathError("XPTY0004", fmt.Sprintf("first argument of format-date must be xs:date, got %T", args[0][0]))
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	result, err := formatDateTimePicture(time.Time(dateVal), picture, "date")
	if err != nil {
		return nil, err
	}
	return Sequence{result}, nil
}

func fnFormatDateTime(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{""}, nil
	}
	dtVal, ok := args[0][0].(XSDateTime)
	if !ok {
		return nil, NewXPathError("XPTY0004", fmt.Sprintf("first argument of format-dateTime must be xs:dateTime, got %T", args[0][0]))
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	result, err := formatDateTimePicture(time.Time(dtVal), picture, "dateTime")
	if err != nil {
		return nil, err
	}
	return Sequence{result}, nil
}

func fnFormatTime(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return Sequence{""}, nil
	}
	tVal, ok := args[0][0].(XSTime)
	if !ok {
		return nil, NewXPathError("XPTY0004", fmt.Sprintf("first argument of format-time must be xs:time, got %T", args[0][0]))
	}
	picture, err := StringValue(args[1])
	if err != nil {
		return nil, err
	}
	result, err := formatDateTimePicture(time.Time(tVal), picture, "time")
	if err != nil {
		return nil, err
	}
	return Sequence{result}, nil
}

// formatDateTimePicture formats a time.Time according to an XPath picture string.
// Handles all component specifiers: Y, M, D, d, F, W, w, H, h, m, s, f, Z, z, P, C.
// dateComponents defines which component specifiers are valid for each type.
var dateComponents = map[string]string{
	"date":     "YMDdFWwECZz",
	"time":     "HhmsffPZz",
	"dateTime": "YMDdFWwHhmsffPECZz",
}

func formatDateTimePicture(t time.Time, picture string, typ string) (string, error) {
	validComponents := dateComponents[typ]
	var result strings.Builder
	runes := []rune(picture)
	i := 0
	for i < len(runes) {
		if runes[i] == '[' {
			end := i + 1
			for end < len(runes) && runes[end] != ']' {
				end++
			}
			if end >= len(runes) {
				result.WriteRune('[')
				i++
				continue
			}
			component := strings.TrimSpace(string(runes[i+1 : end]))
			i = end + 1

			if len(component) == 0 {
				continue
			}

			// Escaped bracket: [[ → [
			if component == "[" {
				result.WriteRune('[')
				continue
			}

			specifier := rune(component[0])

			// Validate component is known
			knownComponents := "YMDdFWwHhmsffPECZz"
			if !strings.ContainsRune(knownComponents, specifier) {
				return "", NewXPathError("FOFD1340", fmt.Sprintf("invalid component specifier [%s]", component))
			}

			// Validate component is available for this type (FOFD1350)
			if !strings.ContainsRune(validComponents, specifier) {
				return "", NewXPathError("FOFD1350", fmt.Sprintf("component [%c] not available for %s", specifier, typ))
			}

			rawModifier := strings.TrimSpace(component[1:])

			// Check for ordinal suffix ('o', 'c', 't' at end of modifier)
			isOrdinal := false
			if len(rawModifier) > 0 {
				lastChar := rawModifier[len(rawModifier)-1]
				if lastChar == 'o' {
					isOrdinal = true
				}
			}

			// Parse modifier: [presentation][,width]
			presentation, minWidth, maxWidth := parseComponentModifier(rawModifier)

			val := formatDateTimeComponent(t, specifier, presentation, minWidth, maxWidth)
			if isOrdinal && len(val) > 0 {
				// Add ordinal suffix to numeric output
				if n, err := strconv.Atoi(val); err == nil {
					val += ordinalSuffix(n)
				}
			}
			result.WriteString(val)
		} else if runes[i] == ']' {
			// ]] → ]
			if i+1 < len(runes) && runes[i+1] == ']' {
				result.WriteRune(']')
				i += 2
			} else {
				result.WriteRune(']')
				i++
			}
		} else {
			result.WriteRune(runes[i])
			i++
		}
	}
	return result.String(), nil
}

// parseComponentModifier parses an XPath date/time component modifier string.
// Input: the part after the specifier letter, e.g. "01", "Nn", "1,3-4", "001"
// Returns: presentation modifier, minWidth, maxWidth (0 means default)
func parseComponentModifier(mod string) (string, int, int) {
	// Strip ordinal/cardinal suffix: trailing 'o', 'c', 't' (after digits/letters)
	// e.g., "1o" → "1" with ordinal, "0001o" → "0001" with ordinal
	// Note: we handle ordinal in the output, not here — just strip it
	// so presentation parsing works correctly
	if len(mod) > 0 {
		last := mod[len(mod)-1]
		if last == 'o' || last == 'c' || last == 't' {
			// Only strip if preceded by a digit or if it's a standalone modifier
			if len(mod) == 1 || (mod[len(mod)-2] >= '0' && mod[len(mod)-2] <= '9') {
				mod = mod[:len(mod)-1]
			}
		}
	}

	// Split at comma for width modifier
	presentation := mod
	minW, maxW := 0, 0
	if idx := strings.LastIndex(mod, ","); idx >= 0 {
		presentation = strings.TrimSpace(mod[:idx])
		widthStr := strings.TrimSpace(mod[idx+1:])
		// Parse min-max
		if before, after, ok := strings.Cut(widthStr, "-"); ok {
			minStr := before
			maxStr := after
			if minStr != "*" {
				minW, _ = strconv.Atoi(minStr)
			}
			if maxStr != "*" {
				maxW, _ = strconv.Atoi(maxStr)
			}
		} else if widthStr != "*" {
			minW, _ = strconv.Atoi(widthStr)
			maxW = minW
		}
	}

	// If presentation is empty, derive minWidth from it
	if presentation == "" && minW == 0 {
		return "", 0, maxW
	}

	// If presentation is all digits like "01", "001", "1", derive minWidth
	if minW == 0 && len(presentation) > 0 {
		allDigits := true
		for _, r := range presentation {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			minW = len(presentation)
		}
	}

	return presentation, minW, maxW
}

func formatDateTimeComponent(t time.Time, spec rune, presentation string, minWidth, maxWidth int) string {
	var val int
	switch spec {
	case 'Y': // year
		val = t.Year()
		if val < 0 {
			val = -val
		}
		// Check for Roman numeral presentation
		if presentation == "I" || presentation == "i" {
			return formatRoman(val, presentation == "I")
		}
		if minWidth == 0 && maxWidth == 0 {
			minWidth = 4
		}
		s := fmt.Sprintf("%0*d", max(minWidth, 1), val)
		// Apply maxWidth: truncate from left
		if maxWidth > 0 && len(s) > maxWidth {
			s = s[len(s)-maxWidth:]
		}
		return s
	case 'M': // month
		val = int(t.Month())
		// Name modifiers
		if strings.Contains(presentation, "N") || strings.Contains(presentation, "n") {
			name := t.Month().String()
			if strings.HasPrefix(presentation, "N") && !strings.Contains(presentation, "n") {
				name = strings.ToUpper(name)
			} else if presentation == "n" {
				name = strings.ToLower(name)
			}
			if maxWidth > 0 && len(name) > maxWidth {
				name = name[:maxWidth]
			}
			return name
		}
		if presentation == "I" || presentation == "i" {
			return formatRoman(val, presentation == "I")
		}
		return formatInt(val, minWidth)
	case 'D': // day of month
		val = t.Day()
		return formatInt(val, minWidth)
	case 'd': // day of year
		val = t.YearDay()
		return formatInt(val, max(minWidth, 1))
	case 'F': // day of week
		if strings.Contains(presentation, "N") || strings.Contains(presentation, "n") || presentation == "" {
			name := t.Weekday().String()
			if strings.HasPrefix(presentation, "N") && !strings.Contains(presentation, "n") {
				name = strings.ToUpper(name)
			} else if presentation == "n" {
				name = strings.ToLower(name)
			}
			if maxWidth > 0 && len(name) > maxWidth {
				name = name[:maxWidth]
			}
			return name
		}
		val = int(t.Weekday())
		if val == 0 {
			val = 7
		}
		return formatInt(val, minWidth)
	case 'W': // week of year
		_, val = t.ISOWeek()
		return formatInt(val, minWidth)
	case 'w': // week in month
		val = (t.Day()-1)/7 + 1
		return formatInt(val, minWidth)
	case 'H': // hour (0-23)
		val = t.Hour()
		if minWidth == 0 && presentation == "" {
			minWidth = 1
		}
		return formatInt(val, minWidth)
	case 'h': // hour (1-12)
		val = t.Hour() % 12
		if val == 0 {
			val = 12
		}
		return formatInt(val, minWidth)
	case 'm': // minute
		val = t.Minute()
		if minWidth == 0 && presentation == "" {
			minWidth = 1
		}
		return formatInt(val, minWidth)
	case 's': // second
		val = t.Second()
		if minWidth == 0 && presentation == "" {
			minWidth = 1
		}
		return formatInt(val, minWidth)
	case 'f': // fractional seconds
		ns := t.Nanosecond()
		// Default: presentation determines precision, or use width
		precision := minWidth
		if precision == 0 {
			precision = 1
		}
		if maxWidth > 0 && maxWidth < precision {
			precision = maxWidth
		}
		frac := fmt.Sprintf("%09d", ns)
		if precision <= 9 {
			frac = frac[:precision]
		}
		// Trim trailing zeros if maxWidth allows fewer
		if maxWidth > 0 && minWidth < maxWidth {
			for len(frac) > minWidth && frac[len(frac)-1] == '0' {
				frac = frac[:len(frac)-1]
			}
		}
		if len(frac) == 0 {
			frac = "0"
		}
		return frac
	case 'Z', 'z': // timezone
		_, offset := t.Zone()
		if offset == 0 {
			if spec == 'Z' {
				return "Z"
			}
			return "+00:00"
		}
		sign := "+"
		if offset < 0 {
			sign = "-"
			offset = -offset
		}
		h := offset / 3600
		m := (offset % 3600) / 60
		return fmt.Sprintf("%s%02d:%02d", sign, h, m)
	case 'P': // am/pm
		am, pm := "a.m.", "p.m."
		switch presentation {
		case "N":
			am, pm = "AM", "PM"
		case "n":
			am, pm = "am", "pm"
		case "Nn":
			am, pm = "Am", "Pm"
		}
		if t.Hour() < 12 {
			return am
		}
		return pm
	case 'C': // calendar
		return "ISO"
	case 'E': // era
		if t.Year() > 0 {
			return "AD"
		}
		return "BC"
	}
	return "[" + string(spec) + presentation + "]"
}

// formatInt formats an integer with minimum width (zero-padded).
func formatInt(val, minWidth int) string {
	if minWidth <= 1 {
		return fmt.Sprintf("%d", val)
	}
	return fmt.Sprintf("%0*d", minWidth, val)
}

// jsonToXPath converts a JSON string to XPath maps/arrays/atomic values.
func jsonToXPath(jsonStr string) (Item, error) {
	var raw any
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return jsonValueToXPath(raw)
}

func jsonValueToXPath(v any) (Item, error) {
	switch val := v.(type) {
	case nil:
		return nil, nil // XPath empty sequence for JSON null
	case bool:
		return val, nil
	case json.Number:
		s := val.String()
		if strings.Contains(s, ".") || strings.Contains(s, "e") || strings.Contains(s, "E") {
			f, err := val.Float64()
			if err != nil {
				return nil, err
			}
			return XSDouble(f), nil
		}
		i, err := val.Int64()
		if err != nil {
			f, err := val.Float64()
			if err != nil {
				return nil, err
			}
			return XSDouble(f), nil
		}
		return int(i), nil
	case string:
		return val, nil
	case []any:
		members := make([]Sequence, len(val))
		for i, elem := range val {
			item, err := jsonValueToXPath(elem)
			if err != nil {
				return nil, err
			}
			if item == nil {
				members[i] = Sequence{} // null → empty sequence as array member
			} else {
				members[i] = Sequence{item}
			}
		}
		return &XPathArray{Members: members}, nil
	case map[string]any:
		var entries []MapEntry
		for k, v := range val {
			item, err := jsonValueToXPath(v)
			if err != nil {
				return nil, err
			}
			var value Sequence
			if item == nil {
				value = Sequence{} // null → empty sequence
			} else {
				value = Sequence{item}
			}
			entries = append(entries, MapEntry{Key: k, Value: value})
		}
		return &XPathMap{Entries: entries}, nil
	}
	return nil, fmt.Errorf("unsupported JSON type: %T", v)
}

// makeRNGMap creates a random-number-generator result map.
func makeRNGMap() *XPathMap {
	permuteFn := &XPathFunction{
		Name: "permute", Namespace: nsFN, Arity: 1,
		Fn: func(ctx *Context, args []Sequence) (Sequence, error) {
			return args[0], nil // identity permutation
		},
	}
	var nextFn *XPathFunction
	nextFn = &XPathFunction{
		Name: "next", Namespace: nsFN, Arity: 0,
		Fn: func(ctx *Context, args []Sequence) (Sequence, error) {
			m := &XPathMap{Entries: []MapEntry{
				{Key: "number", Value: Sequence{XSDouble(0.5)}},
				{Key: "next", Value: Sequence{nextFn}},
				{Key: "permute", Value: Sequence{permuteFn}},
			}}
			return Sequence{m}, nil
		},
	}
	return &XPathMap{Entries: []MapEntry{
		{Key: "number", Value: Sequence{XSDouble(0.5)}},
		{Key: "next", Value: Sequence{nextFn}},
		{Key: "permute", Value: Sequence{permuteFn}},
	}}
}

// isStringLike returns true if the item is a string-like type for comparison purposes.
func isStringLike(itm Item) bool {
	switch itm.(type) {
	case string, XSString, XSAnyURI, XSUntypedAtomic:
		return true
	}
	return false
}

// validateComparableSequence checks that all items in a sequence are of compatible types.
func validateComparableSequence(seq Sequence) error {
	if len(seq) < 2 {
		return nil
	}
	firstIsString := isStringLike(seq[0])
	_, firstIsNumeric := ToFloat64(seq[0])
	_, firstIsDuration := seq[0].(XSDuration)
	for _, itm := range seq[1:] {
		isStr := isStringLike(itm)
		_, isNum := ToFloat64(itm)
		_, isDur := itm.(XSDuration)
		if firstIsString && !isStr && !isNum {
			return NewXPathError("FORG0006", "fn:min/max: incomparable types in sequence")
		}
		if firstIsNumeric && isStr {
			return NewXPathError("FORG0006", "fn:min/max: cannot compare numeric with string")
		}
		if firstIsDuration && !isDur {
			return NewXPathError("FORG0006", "fn:min/max: cannot mix durations with other types")
		}
		if !firstIsString && !firstIsNumeric && !firstIsDuration {
			return NewXPathError("FORG0006", fmt.Sprintf("fn:min/max: uncomparable type %T", seq[0]))
		}
	}
	return nil
}

// buildXPathPath returns the XPath path expression for a node.
func buildXPathPath(node goxml.XMLNode) string {
	switch n := node.(type) {
	case *goxml.XMLDocument:
		return "/"
	case *goxml.Element:
		if n.Parent == nil {
			return "/" + n.Name
		}
		parentPath := ""
		if p, ok := n.Parent.(*goxml.Element); ok {
			parentPath = buildXPathPath(p)
		} else {
			parentPath = ""
		}
		// Find position among same-named siblings
		pos := 1
		if parent, ok := n.Parent.(*goxml.Element); ok {
			for _, child := range parent.Children() {
				if elt, ok := child.(*goxml.Element); ok {
					if elt == n {
						break
					}
					if elt.Name == n.Name {
						pos++
					}
				}
			}
		}
		return fmt.Sprintf("%s/Q{%s}%s[%d]", parentPath, n.Namespaces[n.Prefix], n.Name, pos)
	case goxml.Attribute:
		return "/@" + n.Name
	}
	return ""
}

func fnSort(ctx *Context, args []Sequence) (Sequence, error) {
	seq := args[0]
	if len(seq) <= 1 {
		return seq, nil
	}

	// Get key function if provided (3rd argument)
	var keyFn *XPathFunction
	if len(args) >= 3 && len(args[2]) > 0 {
		if fn, ok := args[2][0].(*XPathFunction); ok {
			keyFn = fn
		}
	}

	type sortEntry struct {
		key Sequence
		itm Item
	}
	entries := make([]sortEntry, len(seq))
	for i, itm := range seq {
		var key Sequence
		if keyFn != nil {
			var err error
			key, err = keyFn.Call(ctx, []Sequence{{itm}})
			if err != nil {
				return nil, err
			}
		} else {
			key = Sequence{itm}
		}
		entries[i] = sortEntry{key: key, itm: itm}
	}

	// Stable sort using comparison
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].key, entries[j].key
		if len(a) == 0 {
			return true
		}
		if len(b) == 0 {
			return false
		}
		// Numeric comparison for numeric types
		if na, ok := toNumber(a[0]); ok {
			if nb, ok := toNumber(b[0]); ok {
				return na < nb
			}
		}
		// String comparison fallback
		sa := itemStringvalue(a[0])
		sb := itemStringvalue(b[0])
		return sa < sb
	})

	result := make(Sequence, len(entries))
	for i, e := range entries {
		result[i] = e.itm
	}
	return result, nil
}

// toNumber tries to convert an item to float64 for comparison.
func toNumber(itm Item) (float64, bool) {
	switch v := itm.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// nodeParent returns the parent of an XMLNode, or nil if it has no parent.
func nodeParent(n goxml.XMLNode) goxml.XMLNode {
	switch v := n.(type) {
	case *goxml.Element:
		return v.Parent
	case goxml.Attribute:
		return v.Parent
	default:
		return nil
	}
}

func fnOutermost(ctx *Context, args []Sequence) (Sequence, error) {
	// Collect all nodes from the input sequence
	var nodes []goxml.XMLNode
	for _, itm := range args[0] {
		if n, ok := itm.(goxml.XMLNode); ok {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return Sequence{}, nil
	}
	// Build a set of node IDs for quick lookup
	nodeIDs := make(map[int]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.GetID()] = true
	}
	// Keep nodes that have no ancestor in the set
	var result goxml.SortByDocumentOrder
	for _, n := range nodes {
		hasAncestorInSet := false
		cur := nodeParent(n)
		for cur != nil {
			if nodeIDs[cur.GetID()] {
				hasAncestorInSet = true
				break
			}
			cur = nodeParent(cur)
		}
		if !hasAncestorInSet {
			result = append(result, n)
		}
	}
	sorted := result.SortAndEliminateDuplicates()
	seq := make(Sequence, len(sorted))
	for i, n := range sorted {
		seq[i] = n
	}
	return seq, nil
}

func fnInnermost(ctx *Context, args []Sequence) (Sequence, error) {
	// Collect all nodes from the input sequence
	var nodes []goxml.XMLNode
	for _, itm := range args[0] {
		if n, ok := itm.(goxml.XMLNode); ok {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return Sequence{}, nil
	}
	// Build a set of node IDs for quick lookup
	nodeIDs := make(map[int]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.GetID()] = true
	}
	// Keep nodes that have no descendant in the set.
	// A node has a descendant in the set if some other node in the set
	// has this node as an ancestor.
	ancestorIDs := make(map[int]bool)
	for _, n := range nodes {
		cur := nodeParent(n)
		for cur != nil {
			id := cur.GetID()
			if ancestorIDs[id] {
				break // already recorded ancestors above this
			}
			ancestorIDs[id] = true
			cur = nodeParent(cur)
		}
	}
	// A node is innermost if its ID is NOT in ancestorIDs
	// (i.e., no other node in the set has it as an ancestor)
	var result goxml.SortByDocumentOrder
	for _, n := range nodes {
		if !ancestorIDs[n.GetID()] {
			result = append(result, n)
		}
	}
	sorted := result.SortAndEliminateDuplicates()
	seq := make(Sequence, len(sorted))
	for i, n := range sorted {
		seq[i] = n
	}
	return seq, nil
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
	RegisterFunction(&Function{Name: "apply", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) != 1 {
			return nil, NewXPathError("XPTY0004", "first argument of fn:apply must be a function")
		}
		fn, ok := args[0][0].(*XPathFunction)
		if !ok {
			return nil, NewXPathError("XPTY0004", fmt.Sprintf("first argument of fn:apply must be a function, got %T", args[0][0]))
		}
		arr, ok := args[1][0].(*XPathArray)
		if !ok {
			return nil, NewXPathError("XPTY0004", "second argument of fn:apply must be an array")
		}
		fnArgs := make([]Sequence, len(arr.Members))
		copy(fnArgs, arr.Members)
		return fn.Call(ctx, fnArgs)
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "concat", Namespace: nsFN, F: fnConcat, MinArg: 2, MaxArg: -1})
	RegisterFunction(&Function{Name: "contains", Namespace: nsFN, F: fnContains, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "contains-token", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{false}, nil
		}
		token, err := StringValue(args[1])
		if err != nil {
			return nil, err
		}
		token = strings.TrimSpace(token)
		if token == "" {
			return Sequence{false}, nil
		}
		for _, itm := range args[0] {
			sv := itemStringvalue(itm)
			if slices.Contains(strings.Fields(sv), token) {
				return Sequence{true}, nil
			}
		}
		return Sequence{false}, nil
	}, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "count", Namespace: nsFN, F: fnCount, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "current-date", Namespace: nsFN, F: fnCurrentDate, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-dateTime", Namespace: nsFN, F: fnCurrentDateTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "current-time", Namespace: nsFN, F: fnCurrentTime, MinArg: 0, MaxArg: 0})
	RegisterFunction(&Function{Name: "data", Namespace: nsFN, F: fnData, MaxArg: 1})
	RegisterFunction(&Function{Name: "deep-equal", Namespace: nsFN, F: fnDeepEqual, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "distinct-values", Namespace: nsFN, F: fnDistinctValues, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "doc", Namespace: nsFN, F: fnDoc, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "doc-available", Namespace: nsFN, F: fnDocAvailable, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "encode-for-uri", Namespace: nsFN, F: fnEncodeForURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "escape-html-uri", Namespace: nsFN, F: fnEscapeHTMLURI, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "empty", Namespace: nsFN, F: fnEmpty, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "for-each", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[1]) != 1 {
			return nil, fmt.Errorf("fn:for-each: second argument must be a single function")
		}
		fn, ok := args[1][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("fn:for-each: second argument must be a function, got %T", args[1][0])
		}
		var result Sequence
		for _, item := range args[0] {
			res, err := fn.Call(ctx, []Sequence{{item}})
			if err != nil {
				return nil, err
			}
			result = append(result, res...)
		}
		return result, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "filter", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[1]) != 1 {
			return nil, fmt.Errorf("fn:filter: second argument must be a single function")
		}
		fn, ok := args[1][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("fn:filter: second argument must be a function, got %T", args[1][0])
		}
		var result Sequence
		for _, item := range args[0] {
			res, err := fn.Call(ctx, []Sequence{{item}})
			if err != nil {
				return nil, err
			}
			if len(res) == 1 {
				if b, ok := res[0].(bool); ok && b {
					result = append(result, item)
				}
			}
		}
		return result, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "fold-left", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[2]) != 1 {
			return nil, fmt.Errorf("fn:fold-left: third argument must be a single function")
		}
		fn, ok := args[2][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("fn:fold-left: third argument must be a function, got %T", args[2][0])
		}
		acc := args[1]
		for _, item := range args[0] {
			var err error
			acc, err = fn.Call(ctx, []Sequence{acc, {item}})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "fold-right", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[2]) != 1 {
			return nil, fmt.Errorf("fn:fold-right: third argument must be a single function")
		}
		fn, ok := args[2][0].(*XPathFunction)
		if !ok {
			return nil, fmt.Errorf("fn:fold-right: third argument must be a function, got %T", args[2][0])
		}
		acc := args[1]
		seq := args[0]
		for i := len(seq) - 1; i >= 0; i-- {
			var err error
			acc, err = fn.Call(ctx, []Sequence{{seq[i]}, acc})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "exactly-one", Namespace: nsFN, F: fnExactlyOne, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "exists", Namespace: nsFN, F: fnExists, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "ends-with", Namespace: nsFN, F: fnEndsWith, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "false", Namespace: nsFN, F: fnFalse})
	RegisterFunction(&Function{Name: "floor", Namespace: nsFN, F: fnFloor, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "function-lookup", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		qname, ok := args[0][0].(XSQName)
		if !ok {
			return nil, NewXPathError("XPTY0004", "first argument of function-lookup must be an xs:QName")
		}
		arity, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		ns := qname.Namespace
		name := qname.Localname
		fn := getfunction(ns, name)
		if fn == nil {
			return Sequence{}, nil
		}
		return Sequence{&XPathFunction{
			Name:      name,
			Namespace: ns,
			Arity:     int(arity),
			Fn:        fn.F,
		}}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "format-date", Namespace: nsFN, F: fnFormatDate, MinArg: 2, MaxArg: 5})
	RegisterFunction(&Function{Name: "format-dateTime", Namespace: nsFN, F: fnFormatDateTime, MinArg: 2, MaxArg: 5})
	RegisterFunction(&Function{Name: "format-time", Namespace: nsFN, F: fnFormatTime, MinArg: 2, MaxArg: 5})
	RegisterFunction(&Function{Name: "format-integer", Namespace: nsFN, F: fnFormatInteger, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "format-number", Namespace: nsFN, F: fnFormatNumber, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "day-from-date", Namespace: nsFN, F: fnDayFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "day-from-dateTime", Namespace: nsFN, F: fnDayFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "days-from-duration", Namespace: nsFN, F: fnDaysFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-dateTime", Namespace: nsFN, F: fnHoursFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-duration", Namespace: nsFN, F: fnHoursFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "hours-from-time", Namespace: nsFN, F: fnHoursFromTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "has-children", Namespace: nsFN, F: fnHasChildren, MaxArg: 1})
	RegisterFunction(&Function{Name: "head", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		return Sequence{args[0][0]}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "index-of", Namespace: nsFN, F: fnIndexOf, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "innermost", Namespace: nsFN, F: fnInnermost, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "json-to-xml", Namespace: nsFN, F: fnJSONToXML, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "xml-to-json", Namespace: nsFN, F: fnXMLToJSON, MinArg: 1, MaxArg: 2})
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
	RegisterFunction(&Function{Name: "nilled", Namespace: nsFN, F: fnNilled, MaxArg: 1})
	RegisterFunction(&Function{Name: "node-name", Namespace: nsFN, F: fnNodeName, MaxArg: 1})
	RegisterFunction(&Function{Name: "namespace-uri", Namespace: nsFN, F: fnNamespaceURI, MaxArg: 1})
	RegisterFunction(&Function{Name: "namespace-uri-for-prefix", Namespace: nsFN, F: fnNamespaceURIForPrefix, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "namespace-uri-from-QName", Namespace: nsFN, F: fnNamespaceURIFromQName, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "not", Namespace: nsFN, F: fnNot, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-space", Namespace: nsFN, F: fnNormalizeSpace, MaxArg: 1})
	RegisterFunction(&Function{Name: "normalize-unicode", Namespace: nsFN, F: fnNormalizeUnicode, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "number", Namespace: nsFN, F: fnNumber, MaxArg: 1})
	RegisterFunction(&Function{Name: "one-or-more", Namespace: nsFN, F: fnOneOrMore, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "outermost", Namespace: nsFN, F: fnOutermost, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "position", Namespace: nsFN, F: fnPosition})
	RegisterFunction(&Function{Name: "prefix-from-QName", Namespace: nsFN, F: fnPrefixFromQName, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "QName", Namespace: nsFN, F: fnQName, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "remove", Namespace: nsFN, F: fnRemove, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "replace", Namespace: nsFN, F: fnReplace, MinArg: 3, MaxArg: 4})
	RegisterFunction(&Function{Name: "resolve-QName", Namespace: nsFN, F: fnResolveQName, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "resolve-uri", Namespace: nsFN, F: fnResolveURI, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "reverse", Namespace: nsFN, F: fnReverse, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "root", Namespace: nsFN, F: fnRoot, MaxArg: 1})
	RegisterFunction(&Function{Name: "round", Namespace: nsFN, F: fnRound, MaxArg: 2})
	RegisterFunction(&Function{Name: "round-half-to-even", Namespace: nsFN, F: fnRoundHalfToEven, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "sort", Namespace: nsFN, F: fnSort, MinArg: 1, MaxArg: 3})
	RegisterFunction(&Function{Name: "string", Namespace: nsFN, F: fnString, MaxArg: 1})
	RegisterFunction(&Function{Name: "starts-with", Namespace: nsFN, F: fnStartsWith, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "string-join", Namespace: nsFN, F: fnStringJoin, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "string-length", Namespace: nsFN, F: fnStringLength, MaxArg: 1})
	RegisterFunction(&Function{Name: "string-to-codepoints", Namespace: nsFN, F: fnStringToCodepoints, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "substring", Namespace: nsFN, F: fnSubstring, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-before", Namespace: nsFN, F: fnSubstringBefore, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "substring-after", Namespace: nsFN, F: fnSubstringAfter, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "subsequence", Namespace: nsFN, F: fnSubsequence, MinArg: 2, MaxArg: 3})
	RegisterFunction(&Function{Name: "sum", Namespace: nsFN, F: fnSum, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "tail", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) <= 1 {
			return Sequence{}, nil
		}
		return args[0][1:], nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "translate", Namespace: nsFN, F: fnTranslate, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "true", Namespace: nsFN, F: fnTrue})
	RegisterFunction(&Function{Name: "tokenize", Namespace: nsFN, F: fnTokenize, MinArg: 1, MaxArg: 3})
	RegisterFunction(&Function{Name: "implicit-timezone", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		_, offset := ctx.CurrentTime().Zone()
		neg := offset < 0
		if neg {
			offset = -offset
		}
		return Sequence{XSDuration{
			Negative: neg,
			Hours:    offset / 3600,
			Minutes:  (offset % 3600) / 60,
		}}, nil
	}})
	RegisterFunction(&Function{Name: "trace", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		// fn:trace returns the first argument unchanged (debug label is ignored)
		return args[0], nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "unordered", Namespace: nsFN, F: fnUnordered, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "upper-case", Namespace: nsFN, F: fnUppercase, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "year-from-date", Namespace: nsFN, F: fnYearFromDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "year-from-dateTime", Namespace: nsFN, F: fnYearFromDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "years-from-duration", Namespace: nsFN, F: fnYearsFromDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "zero-or-one", Namespace: nsFN, F: fnZeroOrOne, MinArg: 1, MaxArg: 1})

	// Tier 1 missing functions
	RegisterFunction(&Function{Name: "unparsed-text", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		href, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(href)
		if err != nil {
			return nil, NewXPathError("FOUT1170", fmt.Sprintf("cannot read %q: %v", href, err))
		}
		return Sequence{string(data)}, nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "unparsed-text-available", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		href, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		_, err = os.Stat(href)
		return Sequence{err == nil}, nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "unparsed-text-lines", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		href, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(href)
		if err != nil {
			return nil, NewXPathError("FOUT1170", fmt.Sprintf("cannot read %q: %v", href, err))
		}
		lines := strings.Split(string(data), "\n")
		var result Sequence
		for _, line := range lines {
			result = append(result, line)
		}
		return result, nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "parse-xml", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		doc, err := goxml.Parse(strings.NewReader(sv))
		if err != nil {
			return nil, NewXPathError("FODC0006", fmt.Sprintf("cannot parse XML: %v", err))
		}
		return Sequence{doc}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "parse-xml-fragment", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		// Wrap in root element to make it valid XML
		doc, err := goxml.Parse(strings.NewReader("<_>" + sv + "</_>"))
		if err != nil {
			return nil, NewXPathError("FODC0006", fmt.Sprintf("cannot parse XML fragment: %v", err))
		}
		return Sequence{doc}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "document-uri", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args) == 0 || len(args[0]) == 0 {
			return Sequence{}, nil
		}
		// document-uri returns empty for most cases in our implementation
		if _, ok := args[0][0].(*goxml.XMLDocument); ok {
			if baseURI, ok := ctx.Store["baseURI"]; ok {
				return Sequence{XSAnyURI(baseURI.(string))}, nil
			}
		}
		return Sequence{}, nil
	}, MaxArg: 1})
	RegisterFunction(&Function{Name: "static-base-uri", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if baseURI, ok := ctx.Store["baseURI"]; ok {
			return Sequence{XSAnyURI(baseURI.(string))}, nil
		}
		return Sequence{}, nil
	}})
	RegisterFunction(&Function{Name: "default-language", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		return Sequence{"en"}, nil
	}})
	RegisterFunction(&Function{Name: "parse-ietf-date", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		sv = strings.TrimSpace(sv)
		// Try various IETF date formats (RFC 2822, RFC 850, asctime)
		formats := []string{
			"Mon, 02 Jan 2006 15:04:05 MST",
			"Mon, 02 Jan 2006 15:04:05 -0700",
			"Mon, 2 Jan 2006 15:04:05 MST",
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"Monday, 02-Jan-06 15:04:05 MST",
			"Monday, 02-Jan-2006 15:04:05 MST",
			"Mon Jan 02 15:04:05 2006",
			"Mon Jan 2 15:04:05 2006",
			"02 Jan 2006 15:04:05 MST",
			"02 Jan 2006 15:04:05 -0700",
			"2 Jan 2006 15:04:05 MST",
			"2 Jan 2006 15:04:05 -0700",
			"Mon, 02 Jan 2006 15:04:05 +0000",
			"Mon 02 Jan 2006 15:04:05 MST",
			"Mon 2 Jan 2006 15:04:05 MST",
			"Mon 02 Jan 2006 15:04:05 -0700",
			"Mon, 02 Jan 2006 15:04 MST",
			"Mon, 02 Jan 2006 15:04 -0700",
			"02 Jan 2006 15:04 MST",
			"02 Jan 2006 15:04 -0700",
			"Mon, 02 Jan 2006 15:04:05",
			"02 Jan 2006 15:04:05",
			"02 Jan 2006",
		}
		// Strip day-of-week prefix variations (Wednesday, Thu, etc.)
		cleaned := sv
		if idx := strings.Index(cleaned, ","); idx >= 0 && idx < 12 {
			cleaned = strings.TrimSpace(cleaned[idx+1:])
		} else {
			// Try removing day name without comma
			days := []string{
				"Monday ", "Tuesday ", "Wednesday ", "Thursday ",
				"Friday ", "Saturday ", "Sunday ",
				"Mon ", "Tue ", "Wed ", "Thu ", "Fri ", "Sat ", "Sun ",
			}
			for _, d := range days {
				if strings.HasPrefix(cleaned, d) {
					cleaned = cleaned[len(d):]
					break
				}
			}
		}
		// Try formats without day-of-week
		noWeekFormats := []string{
			"02 Jan 2006 15:04:05 MST",
			"02 Jan 2006 15:04:05 -0700",
			"2 Jan 2006 15:04:05 MST",
			"2 Jan 2006 15:04:05 -0700",
			"02 Jan 2006 15:04:05 +0000",
			"02 Jan 2006 15:04:05",
			"02 Jan 2006 15:04 MST",
			"02 Jan 2006 15:04 -0700",
			"02 Jan 2006",
			"2 Jan 2006",
			"02-Jan-2006 15:04:05 MST",
			"02-Jan-06 15:04:05 MST",
		}
		for _, f := range formats {
			t, err := time.Parse(f, sv)
			if err == nil {
				return Sequence{XSDateTime(t)}, nil
			}
		}
		for _, f := range noWeekFormats {
			t, err := time.Parse(f, cleaned)
			if err == nil {
				return Sequence{XSDateTime(t)}, nil
			}
		}
		return nil, NewXPathError("FORG0010", fmt.Sprintf("cannot parse IETF date: %q", sv))
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "parse-json", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		sv, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		result, err := jsonToXPath(sv)
		if err != nil {
			return nil, NewXPathError("FOJS0001", fmt.Sprintf("invalid JSON: %v", err))
		}
		return Sequence{result}, nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "json-doc", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		href, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(href)
		if err != nil {
			return nil, NewXPathError("FOJS0001", fmt.Sprintf("cannot read JSON: %v", err))
		}
		result, err := jsonToXPath(string(data))
		if err != nil {
			return nil, NewXPathError("FOJS0001", fmt.Sprintf("invalid JSON: %v", err))
		}
		return Sequence{result}, nil
	}, MinArg: 1, MaxArg: 2})
	RegisterFunction(&Function{Name: "random-number-generator", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		return Sequence{makeRNGMap()}, nil
	}, MaxArg: 1})
	RegisterFunction(&Function{Name: "path", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		var node goxml.XMLNode
		if len(args) == 0 || len(args[0]) == 0 {
			if len(ctx.sequence) > 0 {
				node, _ = ctx.sequence[0].(goxml.XMLNode)
			}
		} else {
			node, _ = args[0][0].(goxml.XMLNode)
		}
		if node == nil {
			return Sequence{}, nil
		}
		return Sequence{buildXPathPath(node)}, nil
	}, MaxArg: 1})

	// Higher-order / introspection functions
	RegisterFunction(&Function{Name: "function-arity", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) != 1 {
			return nil, NewXPathError("XPTY0004", "function-arity requires a single function item")
		}
		fn, ok := args[0][0].(*XPathFunction)
		if !ok {
			return nil, NewXPathError("XPTY0004", "argument is not a function")
		}
		return Sequence{fn.Arity}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "function-name", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) != 1 {
			return nil, NewXPathError("XPTY0004", "function-name requires a single function item")
		}
		fn, ok := args[0][0].(*XPathFunction)
		if !ok {
			return nil, NewXPathError("XPTY0004", "argument is not a function")
		}
		if fn.Name == "" {
			return Sequence{}, nil // anonymous function
		}
		return Sequence{XSQName{Namespace: fn.Namespace, Localname: fn.Name}}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "for-each-pair", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		seq1 := args[0]
		seq2 := args[1]
		if len(args[2]) != 1 {
			return nil, NewXPathError("XPTY0004", "third argument must be a function")
		}
		fn, ok := args[2][0].(*XPathFunction)
		if !ok {
			return nil, NewXPathError("XPTY0004", "third argument must be a function")
		}
		minLen := min(len(seq2), len(seq1))
		var result Sequence
		for i := range minLen {
			r, err := fn.Call(ctx, []Sequence{{seq1[i]}, {seq2[i]}})
			if err != nil {
				return nil, err
			}
			result = append(result, r...)
		}
		return result, nil
	}, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "dateTime", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 || len(args[1]) == 0 {
			return Sequence{}, nil
		}
		d, ok := args[0][0].(XSDate)
		if !ok {
			return nil, NewXPathError("XPTY0004", "first argument must be xs:date")
		}
		t, ok := args[1][0].(XSTime)
		if !ok {
			return nil, NewXPathError("XPTY0004", "second argument must be xs:time")
		}
		dt := time.Time(d)
		tm := time.Time(t)
		combined := time.Date(dt.Year(), dt.Month(), dt.Day(),
			tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(), tm.Location())
		return Sequence{XSDateTime(combined)}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "default-collation", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		return Sequence{"http://www.w3.org/2005/xpath-functions/collation/codepoint"}, nil
	}})
	RegisterFunction(&Function{Name: "error", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		code := "FOER0000"
		desc := ""
		if len(args) > 0 && len(args[0]) > 0 {
			if qn, ok := args[0][0].(XSQName); ok {
				code = qn.Localname
			}
		}
		if len(args) > 1 && len(args[1]) > 0 {
			desc, _ = StringValue(args[1])
		}
		xe := &XPathError{Code: code, Description: desc}
		if len(args) > 2 {
			xe.Value = args[2]
		}
		return nil, xe
	}, MaxArg: 3})
	RegisterFunction(&Function{Name: "environment-variable", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		name, err := StringValue(args[0])
		if err != nil {
			return nil, err
		}
		val, ok := os.LookupEnv(name)
		if !ok {
			return Sequence{}, nil
		}
		return Sequence{val}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "available-environment-variables", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		var result Sequence
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			result = append(result, parts[0])
		}
		return result, nil
	}})
	RegisterFunction(&Function{Name: "generate-id", Namespace: nsFN, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		var node goxml.XMLNode
		if len(args) == 0 || len(args[0]) == 0 {
			if len(ctx.sequence) > 0 {
				if n, ok := ctx.sequence[0].(goxml.XMLNode); ok {
					node = n
				}
			}
		} else if n, ok := args[0][0].(goxml.XMLNode); ok {
			node = n
		}
		if node == nil {
			return Sequence{""}, nil
		}
		return Sequence{fmt.Sprintf("d%d", node.GetID())}, nil
	}, MaxArg: 1})

	// XPath 3.1 math functions (http://www.w3.org/2005/xpath-functions/math)
	RegisterFunction(&Function{Name: "pi", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		return Sequence{math.Pi}, nil
	}})
	RegisterFunction(&Function{Name: "sqrt", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Sqrt(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "exp", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Exp(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "exp10", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Pow(10, n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "log", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Log(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "log10", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Log10(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "pow", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		base, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		exp, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Pow(base, exp)}, nil
	}, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "sin", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Sin(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "cos", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Cos(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "tan", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Tan(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "asin", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Asin(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "acos", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Acos(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "atan", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		n, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Atan(n)}, nil
	}, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "atan2", Namespace: nsMath, F: func(ctx *Context, args []Sequence) (Sequence, error) {
		if len(args[0]) == 0 {
			return Sequence{}, nil
		}
		y, err := NumberValue(args[0])
		if err != nil {
			return nil, err
		}
		x, err := NumberValue(args[1])
		if err != nil {
			return nil, err
		}
		return Sequence{math.Atan2(y, x)}, nil
	}, MinArg: 2, MaxArg: 2})
}

// Function represents an XPath function
type Function struct {
	Name             string
	Namespace        string
	F                func(*Context, []Sequence) (Sequence, error)
	MinArg           int
	MaxArg           int
	DynamicCallError string // if non-empty, dynamic calls (via function reference) raise this error
}

// RegisterFunction registers an XPath function
func RegisterFunction(f *Function) {
	xpathfunctions[f.Namespace+" "+f.Name] = f
}

func getfunction(namespace, name string) *Function {
	return xpathfunctions[namespace+" "+name]
}

// FunctionExists returns true if a function with the given namespace and local name is registered.
func FunctionExists(namespace, name string) bool {
	return xpathfunctions[namespace+" "+name] != nil
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
