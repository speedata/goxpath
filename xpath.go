package goxpath

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/speedata/goxml"
	"golang.org/x/net/html"
)

var exprCache sync.Map // cached parsed XPath expressions: string → EvalFunc

// ErrSequence is raised when a sequence of items is not allowed as an argument.
var ErrSequence = fmt.Errorf("a sequence with more than one item is not allowed here")

// Context is needed for variables, namespaces and XML navigation.
type Context struct {
	Namespaces     map[string]string // Storage for (private) name spaces
	Store          map[any]any       // Store can be used for private variables accessible in functions
	Pos            int               // Used to determine the position() in the sequence
	vars           map[string]Sequence
	sequence       Sequence
	currentItem    Item // XSLT current() item — stable across predicate evaluation
	ctxPositions   []int
	ctxLengths     []int
	size           int
	xmldoc         *goxml.XMLDocument
	decimalFormats map[string]*DecimalFormat
	currentTime    *time.Time // cached per-evaluation, set on first access
	// DefaultCollation is the static default collation, used by string operators
	// and by string functions when no explicit collation argument is supplied.
	// If nil, the Unicode codepoint collation is used.
	DefaultCollation Collation
}

// Collation returns the static default collation, falling back to the
// Unicode codepoint collation if none is set.
func (ctx *Context) Collation() Collation {
	if ctx.DefaultCollation != nil {
		return ctx.DefaultCollation
	}
	return CodepointCollation()
}

// CurrentTime returns the stable current time for this evaluation.
// Per XPath spec, current-dateTime() returns the same value within a single evaluation.
// The time is cached on first access and reused for all subsequent calls.
func (ctx *Context) CurrentTime() time.Time {
	if ctx.currentTime == nil {
		t := currentTimeGetter()
		ctx.currentTime = &t
	}
	return *ctx.currentTime
}

// NewContext returns a context from the xml document
func NewContext(doc *goxml.XMLDocument) *Context {
	ctx := &Context{
		xmldoc:     doc,
		vars:       make(map[string]Sequence),
		Namespaces: make(map[string]string),
	}
	ctx.Namespaces["fn"] = nsFN
	ctx.Namespaces["xs"] = nsXS
	ctx.Namespaces["math"] = nsMath
	ctx.Namespaces["map"] = nsMap
	ctx.Namespaces["array"] = nsArray
	return ctx
}

// CopyContext creates a new context with the underlying xml document but can be
// changed without changing the original context.
func CopyContext(cur *Context) *Context {
	ctx := &Context{
		xmldoc:       cur.xmldoc,
		vars:         maps.Clone(cur.vars),
		Namespaces:   maps.Clone(cur.Namespaces),
		Store:        maps.Clone(cur.Store),
		sequence:     cur.sequence,
		currentItem:  cur.currentItem,
		Pos:          cur.Pos,
		ctxLengths:       slices.Clone(cur.ctxLengths),
		ctxPositions:     slices.Clone(cur.ctxPositions),
		DefaultCollation: cur.DefaultCollation,
	}
	return ctx
}

// ResetFrom reuses an existing context by copying state from src.
// Unlike CopyContext, it reuses the existing map allocations instead of creating
// new maps, which reduces GC pressure in tight loops.
func (ctx *Context) ResetFrom(src *Context) {
	ctx.xmldoc = src.xmldoc
	ctx.currentItem = src.currentItem
	ctx.Pos = src.Pos
	ctx.sequence = src.sequence
	ctx.size = src.size

	clear(ctx.vars)
	maps.Copy(ctx.vars, src.vars)
	clear(ctx.Namespaces)
	maps.Copy(ctx.Namespaces, src.Namespaces)
	clear(ctx.Store)
	maps.Copy(ctx.Store, src.Store)
	ctx.ctxLengths = append(ctx.ctxLengths[:0], src.ctxLengths...)
	ctx.ctxPositions = append(ctx.ctxPositions[:0], src.ctxPositions...)
	ctx.DefaultCollation = src.DefaultCollation
}

// SetContextSequence sets the context sequence and returns the previous one.
func (ctx *Context) SetContextSequence(seq Sequence) Sequence {
	oldCtx := ctx.sequence
	ctx.sequence = seq
	return oldCtx
}

// GetContextSequence returns the current context.
func (ctx *Context) GetContextSequence() Sequence {
	return ctx.sequence
}

// SetCurrentItem sets the XSLT current() item.
func (ctx *Context) SetCurrentItem(item Item) {
	ctx.currentItem = item
}

// CurrentItem returns the XSLT current() item.
func (ctx *Context) CurrentItem() Item {
	return ctx.currentItem
}

// SetSize sets the context size used by last().
func (ctx *Context) SetSize(n int) {
	ctx.size = n
}

// Size returns the context size used by last().
func (ctx *Context) Size() int {
	return ctx.size
}

// Document moves the node navigator to the document and retuns it
func (ctx *Context) Document() goxml.XMLNode {
	ctx.sequence = Sequence{ctx.xmldoc}
	ctx.ctxPositions = nil
	ctx.ctxLengths = nil
	return ctx.xmldoc
}

// Root moves the node navigator to the root node of the document.
func (ctx *Context) Root() (Sequence, error) {
	var err error
	cur, err := ctx.xmldoc.Root()
	if err != nil {
		return nil, err
	}
	ctx.sequence = Sequence{cur}
	ctx.ctxPositions = nil
	ctx.ctxLengths = nil
	return ctx.sequence, err
}

type (
	testFunc           func(*Context, Item) bool
	testfuncChildren   func(*goxml.Element) bool
	testfuncAttributes func(*goxml.Attribute) bool
)

// Current returns all elements in the context that satisfy the testfunc.
func (ctx *Context) Current(tf testfuncChildren) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	ctx.ctxLengths = []int{}
	pos := 0
	l := 0
	for _, n := range ctx.sequence {
		if elt, ok := n.(*goxml.Element); ok {
			if tf(elt) {
				pos++
				l++
				ctx.ctxPositions = append(ctx.ctxPositions, pos)
				seq = append(seq, n)
			}
		}
	}
	for i := 0; i < l; i++ {
		ctx.ctxLengths = append(ctx.ctxLengths, l)
	}

	ctx.sequence = seq
	return seq, nil
}

// Attributes returns all attributes of the current node that satisfy the testfunc
func (ctx *Context) Attributes(tf testfuncAttributes) (Sequence, error) {
	var seq Sequence
	ctx.ctxPositions = []int{}
	for _, n := range ctx.sequence {
		if attr, ok := n.(*goxml.Attribute); ok {
			if tf(attr) {
				ctx.ctxPositions = append(ctx.ctxPositions, 1)
				seq = append(seq, attr)
			}
		}
	}
	ctx.sequence = seq
	return seq, nil
}

func isElement(ctx *Context, itm Item) bool {
	if _, ok := itm.(*goxml.Element); ok {
		return true
	}
	return false
}

func isNode(ctx *Context, itm Item) bool {
	return true
}

func isAttribute(ctx *Context, itm Item) bool {
	if _, ok := itm.(*goxml.Attribute); ok {
		return true
	}
	return false
}

func isComment(ctx *Context, itm Item) bool {
	if _, ok := itm.(goxml.Comment); ok {
		return true
	}
	return false
}

func isProcessingInstruction(ctx *Context, itm Item) bool {
	if _, ok := itm.(goxml.ProcInst); ok {
		return true
	}
	return false
}

func returnProcessingInstructionNameTest(name string) func(*Context, Item) bool {
	return func(ctx *Context, itm Item) bool {
		if pi, ok := itm.(goxml.ProcInst); ok {
			if pi.Target == name {
				return true
			}
		}
		return false
	}
}

func returnAttributeNameTest(name string) func(*Context, Item) bool {
	return func(ctx *Context, itm Item) bool {
		if attr, ok := itm.(*goxml.Attribute); ok {
			if attr.Name == name {
				return true
			}
		}
		return false
	}
}

// returnElementEQNameTest creates a test function for element(Q{namespace}localname).
// The eqname format is "namespace}localname".
func returnElementEQNameTest(eqname string) func(*Context, Item) bool {
	before, after, _ := strings.Cut(eqname, "}")
	ns := before
	localName := after
	return func(ctx *Context, itm Item) bool {
		if elt, ok := itm.(*goxml.Element); ok {
			if elt.Name != localName {
				return false
			}
			eltNS := elt.Namespaces[elt.Prefix]
			return eltNS == ns
		}
		return false
	}
}

func returnElementNameTest(name string) func(*Context, Item) bool {
	// Pre-split the name once instead of on every call.
	parts := strings.SplitN(name, ":", 2)
	var prefix, localName string
	if len(parts) == 2 {
		prefix = parts[0]
		localName = parts[1]
	} else {
		localName = parts[0]
	}

	return func(ctx *Context, itm Item) bool {
		if elt, ok := itm.(*goxml.Element); ok {
			if elt.Name != localName {
				return false
			}
			if prefix != "" {
				return elt.Namespaces[elt.Prefix] == ctx.Namespaces[prefix]
			}
			return true
		}
		return false
	}
}

// Filter applies predicates to the context
func (ctx *Context) Filter(filter EvalFunc) (Sequence, error) {
	var result Sequence
	var lengths []int
	var positions []int
	if ctx.ctxPositions != nil {
		positions = ctx.ctxPositions
		lengths = ctx.ctxLengths
	} else {
		n := len(ctx.sequence)
		positions = make([]int, n)
		lengths = make([]int, n)
		for i := range n {
			positions[i] = i + 1
			lengths[i] = n
		}
	}

	copyContext := ctx.sequence

	for i, itm := range copyContext {
		ctx.sequence = Sequence{itm}
		ctx.Pos = positions[i]
		if len(lengths) > i {
			ctx.size = lengths[i]
		} else {
			ctx.size = 1
		}
		predicate, err := filter(ctx)
		if err != nil {
			return nil, err
		}

		// On first item, check if the predicate returns a number.
		// [1] is the same as "position() = 1"
		if i == 0 && len(predicate) == 1 {
			var predicateNum int
			var isNum bool
			if p0, ok := ToFloat64(predicate[0]); ok {
				predicateNum = int(p0)
				isNum = true
			}
			if isNum {
				var seq Sequence
				for j, jitm := range copyContext {
					if predicateNum == positions[j] {
						seq = append(seq, jitm)
					}
				}
				ctx.sequence = seq
				return seq, nil
			}
		}

		evalItem, err := BooleanValue(predicate)
		if err != nil {
			return nil, err
		}
		if evalItem {
			result = append(result, itm)
		}
	}
	ctx.size = len(result)
	if len(result) == 0 {
		result = Sequence{}
	}
	ctx.sequence = result
	return result, nil
}

// An Item can hold anything such as a number, a string or a node.
type Item any

// ItemStringvalue returns the string value of an individual item.
func ItemStringvalue(itm Item) string {
	return itemStringvalue(itm)
}

// formatXSDoubleString formats a float64 as xs:double/xs:float canonical string.
// Handles INF, -INF, NaN, -0, and uses scientific notation for large/small values.
func formatXSDoubleString(t float64) string {
	if math.IsInf(t, 1) {
		return "INF"
	} else if math.IsInf(t, -1) {
		return "-INF"
	} else if math.IsNaN(t) {
		return "NaN"
	} else if t == 0 {
		if math.Signbit(t) {
			return "-0"
		}
		return "0"
	}
	abs := math.Abs(t)
	if abs >= 1e6 || (abs < 1e-6 && abs != 0) {
		return formatXPathDouble(t)
	}
	return strconv.FormatFloat(t, 'f', -1, 64)
}

// formatXPathDouble formats a float64 in XPath canonical scientific notation
// (no '+' in exponent, no zero-padded exponent: "1.5E2" not "1.5E+02").
func formatXPathDouble(f float64) string {
	s := strconv.FormatFloat(f, 'E', -1, 64)
	s = strings.Replace(s, "E+", "E", 1)
	// Remove leading zeros in exponent: E09 → E9, E-03 → E-3
	if idx := strings.IndexByte(s, 'E'); idx >= 0 {
		mantissa := s[:idx]
		exp := s[idx+1:]
		sign := ""
		if len(exp) > 0 && exp[0] == '-' {
			sign = "-"
			exp = exp[1:]
		}
		exp = strings.TrimLeft(exp, "0")
		if exp == "" {
			exp = "0"
		}
		// Ensure mantissa has decimal point: -1E7 → -1.0E7
		if !strings.Contains(mantissa, ".") {
			mantissa += ".0"
		}
		s = mantissa + "E" + sign + exp
	}
	return s
}

func itemStringvalue(itm Item) string {
	var ret string
	switch t := itm.(type) {
	case XSDouble:
		ret = formatXSDoubleString(float64(t))
	case XSFloat:
		ret = formatXSDoubleString(float64(t))
	case XSDecimal:
		// xs:decimal always uses fixed-point notation, never scientific
		ret = strconv.FormatFloat(float64(t), 'f', -1, 64)
	case float64:
		// Bare float64 — legacy path, format as double
		ret = formatXSDoubleString(t)
	case int:
		ret = strconv.Itoa(t)
	case XSInteger:
		ret = strconv.Itoa(t.V)
	case XSString:
		ret = t.V
	case []uint8:
		ret = string(t)
	case *goxml.Attribute:
		ret = t.Value
	case *goxml.Element:
		ret = t.Stringvalue()
	case *goxml.XMLDocument:
		ret = t.Stringvalue()
	case goxml.Comment:
		ret = t.Contents
	case goxml.ProcInst:
		ret = string(t.Inst)
	case *goxml.ProcInst:
		ret = string(t.Inst)
	case goxml.CharData:
		ret = t.Contents
	case []goxml.XMLNode:
		var str strings.Builder
		for _, n := range t {
			str.WriteString(itemStringvalue(n))
		}
		ret = str.String()
	case string:
		ret = t
	case XSDateTime:
		ret = formatXSDateTime(time.Time(t))
	case XSDate:
		ret = formatXSDate(time.Time(t))
	case XSTime:
		ret = formatXSTime(time.Time(t))
	case XSDuration:
		ret = t.String()
	case XSGYear:
		ret = string(t)
	case XSGMonth:
		ret = string(t)
	case XSGDay:
		ret = string(t)
	case XSGYearMonth:
		ret = string(t)
	case XSGMonthDay:
		ret = string(t)
	case XSAnyURI:
		ret = string(t)
	case XSUntypedAtomic:
		ret = string(t)
	case XSHexBinary:
		ret = string(t)
	case XSBase64Binary:
		ret = string(t)
	case XSQName:
		ret = t.String()
	case *html.Node:
		var buf strings.Builder
		html.Render(&buf, t)
		ret = buf.String()
	default:
		ret = fmt.Sprint(t)
	}
	return ret
}

// A Sequence is a list of Items
type Sequence []Item

func (s Sequence) String() string {
	var sb strings.Builder
	sb.WriteString(`( `)
	for _, itm := range s {
		fmt.Fprintf(&sb, "%v ", itm)
	}
	sb.WriteString(`)`)
	return sb.String()
}

// Stringvalue returns the concatenation of the string value of each item.
func (s Sequence) Stringvalue() string {
	var sb strings.Builder
	for _, itm := range s {
		sb.WriteString(itemStringvalue(itm))
	}
	return sb.String()
}

// StringvalueJoin returns the string values of all items joined by sep.
func (s Sequence) StringvalueJoin(sep string) string {
	if len(s) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, itm := range s {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(itemStringvalue(itm))
	}
	return sb.String()
}

// IntValue returns the sequence value as an integer.
func (s Sequence) IntValue() (int, error) {
	if len(s) > 1 {
		return 0, fmt.Errorf("at most one item expected in the sequence")
	}
	if len(s) == 0 {
		return 0, nil
	}
	numberF, err := strconv.ParseFloat(itemStringvalue(s[0]), 64)
	if err != nil {
		return 0, err
	}
	return int(numberF), nil
}

// EvalFunc returns a sequence evaluating the XPath expression in the given
// context.
type EvalFunc func(*Context) (Sequence, error)

func doCompareString(op string, a, b string) (bool, error) {
	switch op {
	case "<", "lt":
		return a < b, nil
	case "=", "eq":
		return a == b, nil
	case ">", "gt":
		return a > b, nil
	case ">=", "ge":
		return a >= b, nil
	case "<=", "le":
		return a <= b, nil
	case "!=", "ne":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

// formatXSDateTime formats a time.Time as xs:dateTime canonical form.
func formatXSDateTime(t time.Time) string {
	// Check if timezone was set (zero value of Location is UTC for parsed values)
	_, offset := t.Zone()
	base := t.Format("2006-01-02T15:04:05")
	// Add fractional seconds only if non-zero
	if ns := t.Nanosecond(); ns > 0 {
		frac := strings.TrimRight(fmt.Sprintf(".%09d", ns), "0")
		base += frac
	}
	if offset == 0 && t.Location() == time.UTC {
		return base + "Z"
	}
	return base + formatTimezone(offset)
}

// formatXSDate formats a time.Time as xs:date canonical form.
func formatXSDate(t time.Time) string {
	_, offset := t.Zone()
	base := t.Format("2006-01-02")
	if t.Location() == time.UTC {
		// Only add timezone if it was explicitly set
		name, _ := t.Zone()
		if name == "UTC" && offset == 0 {
			// Check if timezone was part of the original parse
			// For dates parsed without timezone, don't add one
		}
	}
	return base
}

// formatXSTime formats a time.Time as xs:time canonical form.
func formatXSTime(t time.Time) string {
	_, offset := t.Zone()
	base := t.Format("15:04:05")
	if ns := t.Nanosecond(); ns > 0 {
		frac := strings.TrimRight(fmt.Sprintf(".%09d", ns), "0")
		base += frac
	}
	if offset == 0 && t.Location() == time.UTC {
		return base + "Z"
	}
	if t.Location() != time.UTC {
		return base + formatTimezone(offset)
	}
	return base
}

func formatTimezone(offsetSec int) string {
	if offsetSec == 0 {
		return "Z"
	}
	sign := "+"
	if offsetSec < 0 {
		sign = "-"
		offsetSec = -offsetSec
	}
	hours := offsetSec / 3600
	minutes := (offsetSec % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

// addItems performs addition or subtraction of two items, handling numeric, duration, and date/time types.
func addItems(a, b Item, op string) (Item, error) {
	// Duration + Duration
	if da, ok := a.(XSDuration); ok {
		if db, ok := b.(XSDuration); ok {
			return addDurations(da, db, op), nil
		}
	}
	// DateTime/Date/Time + Duration
	if dt, ok := a.(XSDateTime); ok {
		if dur, ok := b.(XSDuration); ok {
			return XSDateTime(addTimeDuration(time.Time(dt), dur, op)), nil
		}
		// DateTime - DateTime = Duration
		if dt2, ok := b.(XSDateTime); ok {
			if op == "-" {
				return subtractTimes(time.Time(dt), time.Time(dt2)), nil
			}
		}
	}
	if dt, ok := a.(XSDate); ok {
		if dur, ok := b.(XSDuration); ok {
			return XSDate(addTimeDuration(time.Time(dt), dur, op)), nil
		}
		if dt2, ok := b.(XSDate); ok {
			if op == "-" {
				return subtractTimes(time.Time(dt), time.Time(dt2)), nil
			}
		}
	}
	if dt, ok := a.(XSTime); ok {
		if dur, ok := b.(XSDuration); ok {
			return XSTime(addTimeDuration(time.Time(dt), dur, op)), nil
		}
		if dt2, ok := b.(XSTime); ok {
			if op == "-" {
				return subtractTimes(time.Time(dt), time.Time(dt2)), nil
			}
		}
	}

	// Fall back to numeric with type promotion
	na, err := NumberValue(Sequence{a})
	if err != nil {
		return nil, err
	}
	nb, err := NumberValue(Sequence{b})
	if err != nil {
		return nil, err
	}
	resultType := PromoteNumeric(NumericType(a), NumericType(b))
	var result float64
	if op == "+" {
		result = na + nb
	} else {
		result = na - nb
	}
	return WrapNumeric(result, resultType), nil
}

// addDurations adds or subtracts two durations.
func addDurations(a, b XSDuration, op string) XSDuration {
	sa := durationToMonthsAndSeconds(a)
	sb := durationToMonthsAndSeconds(b)
	if op == "-" {
		sb.months = -sb.months
		sb.seconds = -sb.seconds
	}
	totalMonths := sa.months + sb.months
	totalSeconds := sa.seconds + sb.seconds
	return monthsAndSecondsToDuration(totalMonths, totalSeconds)
}

type monthsSeconds struct {
	months  int
	seconds float64
}

func durationToMonthsAndSeconds(d XSDuration) monthsSeconds {
	months := d.Years*12 + d.Months
	seconds := float64(d.Days)*86400 + float64(d.Hours)*3600 + float64(d.Minutes)*60 + d.Seconds
	if d.Negative {
		months = -months
		seconds = -seconds
	}
	return monthsSeconds{months, seconds}
}

func monthsAndSecondsToDuration(months int, seconds float64) XSDuration {
	var d XSDuration
	if months < 0 || (months == 0 && seconds < 0) {
		d.Negative = true
		months = -months
		seconds = -seconds
	}
	d.Years = months / 12
	d.Months = months % 12
	totalSecs := int(seconds)
	d.Days = totalSecs / 86400
	totalSecs %= 86400
	d.Hours = totalSecs / 3600
	totalSecs %= 3600
	d.Minutes = totalSecs / 60
	d.Seconds = seconds - float64(d.Days*86400+d.Hours*3600+d.Minutes*60)
	if d.Seconds < 0 {
		d.Seconds = 0
	}
	return d
}

// addTimeDuration adds or subtracts a duration from a time.Time.
func addTimeDuration(t time.Time, dur XSDuration, op string) time.Time {
	months := dur.Years*12 + dur.Months
	days := dur.Days
	secs := time.Duration(dur.Hours)*time.Hour + time.Duration(dur.Minutes)*time.Minute +
		time.Duration(dur.Seconds*float64(time.Second))
	if dur.Negative {
		months = -months
		days = -days
		secs = -secs
	}
	if op == "-" {
		months = -months
		days = -days
		secs = -secs
	}
	t = t.AddDate(0, months, days)
	t = t.Add(secs)
	return t
}

// subtractTimes returns the duration between two time.Time values.
func subtractTimes(a, b time.Time) XSDuration {
	diff := a.Sub(b)
	negative := diff < 0
	if negative {
		diff = -diff
	}
	totalSecs := int(diff.Seconds())
	days := totalSecs / 86400
	totalSecs %= 86400
	hours := totalSecs / 3600
	totalSecs %= 3600
	minutes := totalSecs / 60
	secs := diff.Seconds() - float64(days*86400+hours*3600+minutes*60)
	if secs < 0 {
		secs = 0
	}
	return XSDuration{
		Negative: negative,
		Days:     days,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  secs,
	}
}

// durationToSeconds converts a duration to approximate total seconds for comparison.
func durationToSeconds(d XSDuration) float64 {
	total := float64(d.Years)*365.25*24*3600 +
		float64(d.Months)*30.4375*24*3600 +
		float64(d.Days)*24*3600 +
		float64(d.Hours)*3600 +
		float64(d.Minutes)*60 +
		d.Seconds
	if d.Negative {
		total = -total
	}
	return total
}

func doCompareTime(op string, a, b time.Time) (bool, error) {
	switch op {
	case "=", "eq":
		return a.Equal(b), nil
	case "!=", "ne":
		return !a.Equal(b), nil
	case "<", "lt":
		return a.Before(b), nil
	case "<=", "le":
		return !a.After(b), nil
	case ">", "gt":
		return a.After(b), nil
	case ">=", "ge":
		return !a.Before(b), nil
	}
	return false, fmt.Errorf("unknown operator %s for time comparison", op)
}

func doCompareFloat(op string, a, b float64) (bool, error) {
	switch op {
	case "<", "lt":
		return a < b, nil
	case "=", "eq":
		return a == b, nil
	case ">", "gt":
		return a > b, nil
	case ">=", "ge":
		return a >= b, nil
	case "<=", "le":
		return a <= b, nil
	case "!=", "ne":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

func doCompareInt(op string, a, b int) (bool, error) {
	switch op {
	case "<", "lt":
		return a < b, nil
	case "=", "eq":
		return a == b, nil
	case ">", "gt":
		return a > b, nil
	case ">=", "ge":
		return a >= b, nil
	case "<=", "le":
		return a <= b, nil
	case "!=", "ne":
		return a != b, nil
	}
	return false, fmt.Errorf("unknown operator %s", op)
}

type datatype int

const (
	xUnknown datatype = iota
	xDouble
	xInteger
	xString
	xBoolean
)

func compareFunc(op string, a, b any) (bool, error) {
	var floatLeft, floatRight float64
	var intLeft, intRight int
	var stringLeft, stringRight string
	var boolLeft, boolRight bool
	var dtLeft, dtRight datatype

	var ok bool
	if boolLeft, ok = a.(bool); ok {
		dtLeft = xBoolean
	}
	if boolRight, ok = b.(bool); ok {
		dtRight = xBoolean
	}
	// Numeric detection via ToFloat64 (handles float64, int, XSDouble, XSFloat, XSDecimal)
	if f, ok := ToFloat64(a); ok {
		floatLeft = f
		if _, isInt := a.(int); isInt {
			intLeft = a.(int)
			dtLeft = xInteger
		} else {
			dtLeft = xDouble
		}
	}
	if f, ok := ToFloat64(b); ok {
		floatRight = f
		if _, isInt := b.(int); isInt {
			intRight = b.(int)
			dtRight = xInteger
		} else {
			dtRight = xDouble
		}
	}
	if stringLeft, ok = a.(string); ok {
		dtLeft = xString
	} else if v, ok := a.(XSString); ok {
		stringLeft = v.V
		dtLeft = xString
	} else if v, ok := a.(XSAnyURI); ok {
		stringLeft = string(v)
		dtLeft = xString
	} else if v, ok := a.(XSUntypedAtomic); ok {
		stringLeft = string(v)
		dtLeft = xString
	} else if v, ok := a.(XSHexBinary); ok {
		stringLeft = string(v)
		dtLeft = xString
	} else if v, ok := a.(XSBase64Binary); ok {
		stringLeft = string(v)
		dtLeft = xString
	}
	if stringRight, ok = b.(string); ok {
		dtRight = xString
	} else if v, ok := b.(XSString); ok {
		stringRight = v.V
		dtRight = xString
	} else if v, ok := b.(XSAnyURI); ok {
		stringRight = string(v)
		dtRight = xString
	} else if v, ok := b.(XSUntypedAtomic); ok {
		stringRight = string(v)
		dtRight = xString
	} else if v, ok := b.(XSHexBinary); ok {
		stringRight = string(v)
		dtRight = xString
	} else if v, ok := b.(XSBase64Binary); ok {
		stringRight = string(v)
		dtRight = xString
	}
	if attLeft, ok := a.(*goxml.Attribute); ok {
		dtLeft = xString
		stringLeft = attLeft.Stringvalue()
	}
	if eltLeft, ok := a.(*goxml.Element); ok {
		dtLeft = xString
		stringLeft = eltLeft.Stringvalue()
	}
	if attRight, ok := b.(*goxml.Attribute); ok {
		dtRight = xString
		stringRight = attRight.Stringvalue()
	}
	if eltRight, ok := b.(*goxml.Element); ok {
		dtRight = xString
		stringRight = eltRight.Stringvalue()
	}
	if cdLeft, ok := a.(*goxml.CharData); ok {
		dtLeft = xString
		stringLeft = cdLeft.Contents
	} else if cdLeft, ok := a.(goxml.CharData); ok {
		dtLeft = xString
		stringLeft = cdLeft.Contents
	}
	if cdRight, ok := b.(*goxml.CharData); ok {
		dtRight = xString
		stringRight = cdRight.Contents
	} else if cdRight, ok := b.(goxml.CharData); ok {
		dtRight = xString
		stringRight = cdRight.Contents
	}
	if commentLeft, ok := a.(*goxml.Comment); ok {
		dtLeft = xString
		stringLeft = commentLeft.Contents
	} else if commentLeft, ok := a.(goxml.Comment); ok {
		dtLeft = xString
		stringLeft = commentLeft.Contents
	}
	if commentRight, ok := b.(*goxml.Comment); ok {
		dtRight = xString
		stringRight = commentRight.Contents
	} else if commentRight, ok := b.(goxml.Comment); ok {
		dtRight = xString
		stringRight = commentRight.Contents
	}
	if docLeft, ok := a.(*goxml.XMLDocument); ok {
		dtLeft = xString
		stringLeft = docLeft.Stringvalue()
	}
	if docRight, ok := b.(*goxml.XMLDocument); ok {
		dtRight = xString
		stringRight = docRight.Stringvalue()
	}

	if dtLeft == xDouble && dtRight == xDouble {
		return doCompareFloat(op, floatLeft, floatRight)
	}
	if dtLeft == xInteger && dtRight == xInteger {
		return doCompareInt(op, intLeft, intRight)
	}
	if dtLeft == xDouble && dtRight == xInteger {
		// If the float has a fractional part, use float comparison (e.g. 2.5 > 2).
		// Otherwise promote to integer comparison for precision with large numbers.
		if floatLeft != math.Trunc(floatLeft) || math.IsInf(floatLeft, 0) || math.IsNaN(floatLeft) {
			return doCompareFloat(op, floatLeft, float64(intRight))
		}
		return doCompareInt(op, int(floatLeft), intRight)
	}
	if dtLeft == xInteger && dtRight == xDouble {
		if floatRight != math.Trunc(floatRight) || math.IsInf(floatRight, 0) || math.IsNaN(floatRight) {
			return doCompareFloat(op, float64(intLeft), floatRight)
		}
		return doCompareInt(op, intLeft, int(floatRight))
	}
	if dtLeft == xString && dtRight == xString {
		return doCompareString(op, stringLeft, stringRight)
	}
	if dtLeft == xDouble && dtRight == xString {
		var err error
		floatRight, err = strconv.ParseFloat(stringRight, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, floatLeft, floatRight)
	}
	if dtLeft == xString && dtRight == xDouble {
		var err error
		floatLeft, err = strconv.ParseFloat(stringLeft, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, floatLeft, floatRight)
	}
	if dtLeft == xInteger && dtRight == xString {
		var err error
		floatRight, err = strconv.ParseFloat(stringRight, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, float64(intLeft), floatRight)
	}
	if dtLeft == xString && dtRight == xInteger {
		var err error
		floatLeft, err = strconv.ParseFloat(stringLeft, 64)
		if err != nil {
			return false, err
		}
		return doCompareFloat(op, floatLeft, float64(intRight))
	}

	// QName comparisons
	if qa, ok := a.(XSQName); ok {
		if qb, ok := b.(XSQName); ok {
			eq := qa.Namespace == qb.Namespace && qa.Localname == qb.Localname
			switch op {
			case "=", "eq":
				return eq, nil
			case "!=", "ne":
				return !eq, nil
			default:
				return false, NewXPathError("XPTY0004", fmt.Sprintf("QName comparison '%s' not supported", op))
			}
		}
	}

	// g* calendar type comparisons (string-based equality)
	if ga, ok := a.(XSGYear); ok {
		if gb, ok := b.(XSGYear); ok {
			return doCompareString(op, string(ga), string(gb))
		}
	}
	if ga, ok := a.(XSGMonth); ok {
		if gb, ok := b.(XSGMonth); ok {
			return doCompareString(op, string(ga), string(gb))
		}
	}
	if ga, ok := a.(XSGDay); ok {
		if gb, ok := b.(XSGDay); ok {
			return doCompareString(op, string(ga), string(gb))
		}
	}
	if ga, ok := a.(XSGYearMonth); ok {
		if gb, ok := b.(XSGYearMonth); ok {
			return doCompareString(op, string(ga), string(gb))
		}
	}
	if ga, ok := a.(XSGMonthDay); ok {
		if gb, ok := b.(XSGMonthDay); ok {
			return doCompareString(op, string(ga), string(gb))
		}
	}

	// Duration comparisons
	if durLeft, ok := a.(XSDuration); ok {
		if durRight, ok := b.(XSDuration); ok {
			switch op {
			case "=", "eq":
				return durLeft == durRight, nil
			case "!=", "ne":
				return durLeft != durRight, nil
			case "<", "lt":
				return durationToSeconds(durLeft) < durationToSeconds(durRight), nil
			case "<=", "le":
				return durationToSeconds(durLeft) <= durationToSeconds(durRight), nil
			case ">", "gt":
				return durationToSeconds(durLeft) > durationToSeconds(durRight), nil
			case ">=", "ge":
				return durationToSeconds(durLeft) >= durationToSeconds(durRight), nil
			default:
				return false, NewXPathError("XPTY0004", fmt.Sprintf("duration comparison '%s' not supported", op))
			}
		}
	}
	// DateTime/Date/Time comparisons
	if dtLeftVal, ok := a.(XSDateTime); ok {
		if dtRightVal, ok := b.(XSDateTime); ok {
			tl, tr := time.Time(dtLeftVal), time.Time(dtRightVal)
			return doCompareTime(op, tl, tr)
		}
	}
	if dtLeftVal, ok := a.(XSDate); ok {
		if dtRightVal, ok := b.(XSDate); ok {
			tl, tr := time.Time(dtLeftVal), time.Time(dtRightVal)
			return doCompareTime(op, tl, tr)
		}
	}
	if dtLeftVal, ok := a.(XSTime); ok {
		if dtRightVal, ok := b.(XSTime); ok {
			tl, tr := time.Time(dtLeftVal), time.Time(dtRightVal)
			return doCompareTime(op, tl, tr)
		}
	}

	// Boolean comparisons: XPath general comparison promotes the non-boolean
	// operand to boolean via BooleanValue, then compares as booleans.
	if dtLeft == xBoolean || dtRight == xBoolean {
		if dtLeft != xBoolean {
			boolLeft = toBoolForCompare(a, dtLeft, floatLeft, intLeft, stringLeft)
		}
		if dtRight != xBoolean {
			boolRight = toBoolForCompare(b, dtRight, floatRight, intRight, stringRight)
		}
		return doCompareBool(op, boolLeft, boolRight)
	}

	return false, NewXPathError("FORG0001", fmt.Sprintf("cannot compare %T with %T", a, b))
}

// toBoolForCompare converts a non-boolean operand to bool for general comparison.
// XPath spec: number != 0 is true, non-empty string is true.
func toBoolForCompare(v any, dt datatype, floatVal float64, intVal int, stringVal string) bool {
	switch dt {
	case xDouble:
		return floatVal != 0 && !math.IsNaN(floatVal)
	case xInteger:
		return intVal != 0
	case xString:
		return stringVal != ""
	default:
		// For node types or unknown, attempt string conversion
		if s, ok := v.(interface{ Stringvalue() string }); ok {
			return s.Stringvalue() != ""
		}
		return false
	}
}

func doCompareBool(op string, a, b bool) (bool, error) {
	// For ordered comparisons, true > false (true=1, false=0)
	ai, bi := 0, 0
	if a {
		ai = 1
	}
	if b {
		bi = 1
	}
	switch op {
	case "=", "eq":
		return a == b, nil
	case "!=", "ne":
		return a != b, nil
	case "<", "lt":
		return ai < bi, nil
	case "<=", "le":
		return ai <= bi, nil
	case ">", "gt":
		return ai > bi, nil
	case ">=", "ge":
		return ai >= bi, nil
	}
	return false, fmt.Errorf("unknown op %s", op)
}

func isValueComp(op string) bool {
	switch op {
	case "eq", "ne", "lt", "le", "gt", "ge":
		return true
	}
	return false
}

func doCompare(op string, lhs EvalFunc, rhs EvalFunc) (EvalFunc, error) {
	if lhs == nil || rhs == nil {
		return nil, fmt.Errorf("unexpected expression in comparison")
	}
	f := func(ctx *Context) (Sequence, error) {
		savedSeq := ctx.sequence
		left, err := lhs(ctx)
		if err != nil {
			return nil, err
		}
		ctx.sequence = savedSeq
		right, err := rhs(ctx)
		if err != nil {
			return nil, err
		}
		// Value comparisons (eq, ne, ...) return () if either operand is empty,
		// and XPTY0004 if either has more than one item.
		if isValueComp(op) {
			if len(left) == 0 || len(right) == 0 {
				return Sequence{}, nil
			}
			if len(left) > 1 {
				return nil, NewXPathError("XPTY0004", "left operand of value comparison has more than one item")
			}
			if len(right) > 1 {
				return nil, NewXPathError("XPTY0004", "right operand of value comparison has more than one item")
			}
		}
		for _, leftitem := range left {
			for _, rightitem := range right {
				ok, err := compareFunc(op, leftitem, rightitem)
				if err != nil {
					return nil, err
				}
				if ok {
					return Sequence{true}, nil
				}
			}
		}
		return Sequence{false}, nil
	}
	return f, nil
}

func doCompareNode(op string, lhs EvalFunc, rhs EvalFunc) (EvalFunc, error) {
	if lhs == nil || rhs == nil {
		return nil, fmt.Errorf("unexpected expression in node comparison '%s'", op)
	}
	f := func(ctx *Context) (Sequence, error) {
		left, err := lhs(ctx)
		if err != nil {
			return nil, err
		}
		right, err := rhs(ctx)
		if err != nil {
			return nil, err
		}
		if len(left) == 0 || len(right) == 0 {
			return Sequence{}, nil
		}
		if len(left) > 1 {
			return Sequence{}, fmt.Errorf("A sequence of more than one item is not allowed as the first operand of '%s'", op)
		}
		if len(right) > 1 {
			return Sequence{}, fmt.Errorf("A sequence of more than one item is not allowed as the second operand of '%s'", op)
		}
		leftNode, leftOk := left[0].(goxml.XMLNode)
		rightNode, rightOk := right[0].(goxml.XMLNode)
		if !leftOk || !rightOk {
			return nil, fmt.Errorf("operands of '%s' must be nodes", op)
		}

		if op == "is" {
			return Sequence{leftNode.GetID() == rightNode.GetID()}, nil
		}
		if op == "<<" {
			return Sequence{leftNode.GetID() < rightNode.GetID()}, nil
		}
		if op == ">>" {
			return Sequence{leftNode.GetID() > rightNode.GetID()}, nil
		}
		return Sequence{false}, nil
	}
	return f, nil
}

// NumberValue returns the sequence converted to a float.
func NumberValue(s Sequence) (float64, error) {
	if len(s) == 0 {
		return math.NaN(), nil
	}
	if len(s) > 1 {
		return math.NaN(), fmt.Errorf("Required cardinality of first argument of fn:number() is zero or one; supplied value has cardinality more than one")
	}
	firstItem := s[0]
	// Try the fast path: all numeric wrapper types
	if f, ok := ToFloat64(firstItem); ok {
		return f, nil
	}
	if attr, ok := firstItem.(*goxml.Attribute); ok {
		numberF, err := strconv.ParseFloat(attr.Value, 64)
		if err != nil {
			return 0, err
		}
		return numberF, nil
	}
	if elt, ok := firstItem.(*goxml.Element); ok {
		numberF, err := strconv.ParseFloat(elt.Stringvalue(), 64)
		if err != nil {
			return math.NaN(), nil
		}
		return numberF, nil
	}
	if doc, ok := firstItem.(*goxml.XMLDocument); ok {
		numberF, err := strconv.ParseFloat(doc.Stringvalue(), 64)
		if err != nil {
			return math.NaN(), nil
		}
		return numberF, nil
	}
	if cd, ok := firstItem.(goxml.CharData); ok {
		numberF, err := strconv.ParseFloat(strings.TrimSpace(cd.Contents), 64)
		if err != nil {
			return math.NaN(), nil
		}
		return numberF, nil
	}
	if cd, ok := firstItem.(*goxml.CharData); ok {
		numberF, err := strconv.ParseFloat(strings.TrimSpace(cd.Contents), 64)
		if err != nil {
			return math.NaN(), nil
		}
		return numberF, nil
	}

	// Convert string-like types to their string value for numeric parsing
	var str string
	switch v := firstItem.(type) {
	case string:
		str = v
	case XSUntypedAtomic:
		str = string(v)
	default:
		return math.NaN(), nil
	}
	numberF, err := strconv.ParseFloat(strings.TrimSpace(str), 64)
	if err != nil {
		return math.NaN(), nil
	}
	return numberF, nil
}

// BooleanValue returns the effective boolean value of the sequence.
// XPath 2.0 §2.4.3: if the first item is a node, returns true;
// a single atomic value is converted to boolean; otherwise FORG0006.
func BooleanValue(s Sequence) (bool, error) {
	if len(s) == 0 {
		return false, nil
	}
	// If the first item is a node, return true (regardless of sequence length).
	if _, ok := s[0].(goxml.XMLNode); ok {
		return true, nil
	}
	if len(s) == 1 {
		itm := s[0]
		if b, ok := itm.(bool); ok {
			return b, nil
		} else if val, ok := itm.(string); ok {
			return val != "", nil
		} else if val, ok := itm.(XSString); ok {
			return val.V != "", nil
		} else if val, ok := itm.(XSAnyURI); ok {
			return string(val) != "", nil
		} else if val, ok := itm.(XSUntypedAtomic); ok {
			return string(val) != "", nil
		} else if f, ok := ToFloat64(itm); ok {
			// f == f is false if NaN
			return f != 0 && f == f, nil
		}
	}
	return false, NewXPathError("FORG0006", " Invalid argument type")
}

// StringValue returns the string value of the sequence by concatenating the
// string values of each item.
func StringValue(s Sequence) (string, error) {
	var sb strings.Builder
	for _, itm := range s {
		sb.WriteString(itemStringvalue(itm))
	}
	return sb.String(), nil
}

// [2] Expr ::= ExprSingle ("," ExprSingle)*
func parseExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "2 parseExpr")
	var efs []EvalFunc
	for {
		ef, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "2 parseExpr (err)")
			return nil, err
		}
		if ef != nil {
			efs = append(efs, ef)
		}
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read() // comma
	}
	if len(efs) == 1 {
		leaveStep(tl, "2 parseExpr (one ExprSingle)")
		return efs[0], nil
	}
	// more than one ExprSingle

	f := func(ctx *Context) (Sequence, error) {
		var ret Sequence
		for _, ef := range efs {
			seq, err := ef(ctx)
			if err != nil {
				return nil, err
			}
			ret = append(ret, seq...)
		}

		return ret, nil
	}
	leaveStep(tl, "2 parseExpr")
	return f, nil
}

// [3] ExprSingle ::= ForExpr | QuantifiedExpr | IfExpr | OrExpr
func parseExprSingle(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "3 parseExprSingle")
	var ef EvalFunc
	var err error
	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"for", "some", "every", "if", "let"}); ok {
		switch op {
		case "for":
			ef, err = parseForExpr(tl)
		case "some", "every":
			tl.unread()
			ef, err = parseQuantifiedExpr(tl)
		case "if":
			leaveStep(tl, "3 parseExprSingle")
			ef, err = parseIfExpr(tl)
		case "let":
			ef, err = parseLetExpr(tl)
		}
		leaveStep(tl, "3 parseExprSingle")
		return ef, err
	}

	ef, err = parseOrExpr(tl)
	if err != nil {
		leaveStep(tl, "3 parseExprSingle (err)")
		return nil, err
	}
	leaveStep(tl, "3 parseExprSingle")
	return ef, nil
}

// [4] ForExpr ::= SimpleForClause "return" ExprSingle
// [5] SimpleForClause ::= "for" "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)*
func parseForExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "4 parseForExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var err error
	var varnames []string

	for {
		vartoken, err := tl.read()
		if err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		if vn, ok := vartoken.Value.(string); ok {
			varnames = append(varnames, vn)
		} else {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, fmt.Errorf("variable name not a string")
		}
		if err = tl.skipNCName("in"); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		if ef, err = parseExprSingle(tl); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if tl.nexttokIsTyp(tokQName) && tl.nexttokIsValue("return") {
			tl.read()
			break
		}
		if err = tl.skipType(tokComma); err != nil {
			leaveStep(tl, "4 parseForExpr (err)")
			return nil, err
		}
	}

	evalseq, err := parseExprSingle(tl)
	if err != nil {
		leaveStep(tl, "4 parseForExpr (err)")
		return nil, err
	}

	ret := func(ctx *Context) (Sequence, error) {
		var s Sequence
		var err error

		sequences := []Sequence{}
		for _, ef := range efs {
			newcontext := CopyContext(ctx)
			s, err = ef(newcontext)
			if err != nil {
				leaveStep(tl, "4 parseForExpr (err)")
				return nil, err
			}
			sequences = append(sequences, s)
		}
		// go recursively through all variable combinations
		var f func([]string, []Sequence, *Context) (Sequence, error)
		f = func(varnames []string, sequences []Sequence, ctx *Context) (Sequence, error) {
			seq := Sequence{}
			varname := varnames[0]
			sequence := sequences[0]

			for _, itm := range sequence {
				ctx.vars[varname] = Sequence{itm}
				ctx.sequence = Sequence{itm}

				if len(varnames) > 1 {
					s, err := f(varnames[1:], sequences[1:], ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				} else {
					s, err := evalseq(ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				}
			}
			return seq, nil
		}
		var oldValues []Sequence
		for _, vn := range varnames {
			oldValues = append(oldValues, ctx.vars[vn])
		}
		seq, err := f(varnames, sequences, ctx)
		if err != nil {
			return nil, err
		}
		for i, vn := range varnames {
			ctx.vars[vn] = oldValues[i]
		}
		ctx.sequence = seq
		return seq, nil
	}
	leaveStep(tl, "4 parseForExpr")
	return ret, nil
}

// [11] LetExpr ::= SimpleLetClause "return" ExprSingle
// [12] SimpleLetClause ::= "let" SimpleLetBinding ("," SimpleLetBinding)*
// [13] SimpleLetBinding ::= "$" VarName ":=" ExprSingle
func parseLetExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "11 parseLetExpr")

	type letBinding struct {
		varname string
		expr    EvalFunc
	}
	var bindings []letBinding

	for {
		// Read $VarName
		varTok, err := tl.read()
		if err != nil || varTok.Typ != tokVarname {
			leaveStep(tl, "11 parseLetExpr")
			return nil, fmt.Errorf("expected variable name in let expression")
		}
		varname := varTok.Value.(string)

		// Read :=
		if err := tl.skipType(tokOperator); err != nil { // :
			return nil, fmt.Errorf("expected ':=' after variable name in let expression")
		}
		if err := tl.skipType(tokOperator); err != nil { // =
			return nil, fmt.Errorf("expected ':=' after variable name in let expression")
		}

		// Read value expression
		valEf, err := parseExprSingle(tl)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, letBinding{varname: varname, expr: valEf})

		// Check for comma (more bindings) or "return"
		if tl.nexttokIsValue("return") {
			break
		}
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read() // consume comma
	}

	// Read "return"
	if err := tl.skipNCName("return"); err != nil {
		leaveStep(tl, "11 parseLetExpr")
		return nil, fmt.Errorf("expected 'return' in let expression")
	}

	// Read return expression
	returnEf, err := parseExprSingle(tl)
	if err != nil {
		leaveStep(tl, "11 parseLetExpr")
		return nil, err
	}
	if returnEf == nil {
		leaveStep(tl, "11 parseLetExpr")
		return nil, fmt.Errorf("expected expression after 'return' in let expression")
	}

	ef := func(ctx *Context) (Sequence, error) {
		// Save old variable values
		oldValues := make(map[string]Sequence, len(bindings))
		for _, b := range bindings {
			oldValues[b.varname] = ctx.vars[b.varname]
		}
		// Bind new values
		for _, b := range bindings {
			val, err := b.expr(ctx)
			if err != nil {
				// Restore
				for _, b2 := range bindings {
					ctx.vars[b2.varname] = oldValues[b2.varname]
				}
				return nil, err
			}
			ctx.vars[b.varname] = val
		}
		// Evaluate return expression
		result, err := returnEf(ctx)
		// Restore old values
		for _, b := range bindings {
			ctx.vars[b.varname] = oldValues[b.varname]
		}
		return result, err
	}

	leaveStep(tl, "11 parseLetExpr")
	return ef, nil
}

// [6] QuantifiedExpr ::= ("some" | "every") "$" VarName "in" ExprSingle ("," "$" VarName "in" ExprSingle)* "satisfies" ExprSingle
func parseQuantifiedExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "6 parseQuantifiedExpr")
	var efs []EvalFunc
	var varnames []string

	someEvery, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"some", "every"}, tokQName)
	if !ok {
		leaveStep(tl, "6 parseQuantifiedExpr (not some/every)")
		return nil, fmt.Errorf("some or every expected, found %q", someEvery)
	}

	for {
		if !tl.nexttokIsTyp(tokVarname) {
			leaveStep(tl, "6 parseQuantifiedExpr (no var name)")
			return nil, fmt.Errorf("%s: variable name expected", someEvery)
		}

		vartok, err := tl.read()
		if err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (err)")
			return nil, err
		}
		varnames = append(varnames, vartok.Value.(string))

		got, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"in"}, tokQName)
		if !ok {
			leaveStep(tl, "6 parseQuantifiedExpr (missing 'in')")
			return nil, fmt.Errorf("'in' expected, got %s", got)
		}

		ef, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)

		_, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"satisfies"}, tokQName)
		if ok {
			break
		}

		if err = tl.skipType(tokComma); err != nil {
			leaveStep(tl, "6 parseQuantifiedExpr (missing comma)")
			return nil, err
		}

	}

	var err error
	var lastEf EvalFunc
	lastEf, err = parseExprSingle(tl)
	if err != nil {
		leaveStep(tl, "6 parseQuantifiedExpr (err)")
		return nil, err
	}

	evaler := func(ctx *Context) (Sequence, error) {
		var s Sequence
		var err error
		sequences := []Sequence{}
		for _, ef := range efs {
			newcontext := CopyContext(ctx)
			s, err = ef(newcontext)
			if err != nil {
				return nil, err
			}
			sequences = append(sequences, s)
		}
		// go recursively through all variable combinations
		var f func([]string, []Sequence, *Context) (Sequence, error)
		f = func(varnames []string, sequences []Sequence, ctx *Context) (Sequence, error) {
			seq := Sequence{}
			varname := varnames[0]
			sequence := sequences[0]

			for _, itm := range sequence {
				ctx.vars[varname] = Sequence{itm}
				ctx.sequence = Sequence{itm}
				if len(varnames) > 1 {
					s, err := f(varnames[1:], sequences[1:], ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				} else {
					s, err := lastEf(ctx)
					if err != nil {
						return nil, err
					}
					for _, sitm := range s {
						seq = append(seq, sitm)
					}
				}
			}
			return seq, nil
		}
		var oldValues []Sequence
		for _, vn := range varnames {
			oldValues = append(oldValues, ctx.vars[vn])
		}
		seq, err := f(varnames, sequences, ctx)
		if err != nil {
			return nil, err
		}
		for i, vn := range varnames {
			ctx.vars[vn] = oldValues[i]
		}

		if someEvery == "some" {
			for _, itm := range seq {
				bv, err := BooleanValue(Sequence{itm})
				if err != nil {
					return nil, err
				}
				if bv {
					ctx.sequence = Sequence{true}
					goto done
				}
			}
			ctx.sequence = Sequence{false}
		} else {
			for _, itm := range seq {
				bv, err := BooleanValue(Sequence{itm})
				if err != nil {
					return nil, err
				}
				if !bv {
					ctx.sequence = Sequence{false}
					goto done
				}
			}
			ctx.sequence = Sequence{true}
		}
	done:
		return ctx.sequence, nil
	}

	leaveStep(tl, "6 parseQuantifiedExpr")
	return evaler, nil
}

// [7] IfExpr ::= "if" "(" Expr ")" "then" ExprSingle "else" ExprSingle
func parseIfExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "7 parseIfExpr")
	var err error
	var boolEval, thenpart, elsepart EvalFunc

	if err = tl.skipType(tokOpenParen); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		if nexttok, err := tl.peek(); err != nil {
			return nil, fmt.Errorf("open parenthesis expected, found %v", nexttok.Value)
		}
		return nil, fmt.Errorf("open parenthesis expected, found EOF")
	}
	if boolEval, err = parseExpr(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if boolEval == nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, fmt.Errorf("expected condition expression in if")
	}
	if err = tl.skipType(tokCloseParen); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if err = tl.skipNCName("then"); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if thenpart, err = parseExprSingle(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if thenpart == nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, fmt.Errorf("expected expression after 'then'")
	}
	if err = tl.skipNCName("else"); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if elsepart, err = parseExprSingle(tl); err != nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, err
	}
	if elsepart == nil {
		leaveStep(tl, "7 parseIfExpr")
		return nil, fmt.Errorf("expected expression after 'else'")
	}

	f := func(ctx *Context) (Sequence, error) {
		res, err := boolEval(ctx)
		if err != nil {
			return nil, err
		}
		bv, err := BooleanValue(res)
		if err != nil {
			return nil, err
		}
		if bv {
			return thenpart(ctx)
		}
		return elsepart(ctx)
	}
	leaveStep(tl, "7 parseIfExpr")
	return f, nil
}

// [8] OrExpr ::= AndExpr ( "or" AndExpr )*
func parseOrExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "8 parseOrExpr")
	var efs []EvalFunc
	for {
		ef, err := parseAndExpr(tl)
		if err != nil {
			leaveStep(tl, "8 parseOrExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsValue("or") {
			break
		}
		tl.read()
	}

	if len(efs) == 1 {
		leaveStep(tl, "8 parseOrExpr (#efs = 1)")
		return efs[0], nil
	}
	var ef EvalFunc
	ef = func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := BooleanValue(s)
			if err != nil {
				return nil, err
			}
			if b {
				return Sequence{true}, nil
			}

		}
		return Sequence{false}, nil
	}

	leaveStep(tl, "8 parseOrExpr")
	return ef, nil
}

// [9] AndExpr ::= ComparisonExpr ( "and" ComparisonExpr )*
func parseAndExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "9 parseAndExpr")
	var efs []EvalFunc
	for {
		ef, err := parseComparisonExpr(tl)
		if err != nil {
			leaveStep(tl, "9 parseAndExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if !tl.nexttokIsValue("and") {
			break
		}
		tl.read() // and
	}
	if len(efs) == 1 {
		leaveStep(tl, "9 parseAndExpr (#efs == 1)")
		return efs[0], nil
	}

	ef := func(ctx *Context) (Sequence, error) {
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			b, err := BooleanValue(s)
			if err != nil {
				return nil, err
			}
			if !b {
				return Sequence{false}, nil
			}

		}
		return Sequence{true}, nil
	}

	leaveStep(tl, "9 parseAndExpr")
	return ef, nil
}

// [10] ComparisonExpr ::= StringConcatExpr ( (ValueComp | GeneralComp| NodeComp) StringConcatExpr )?
// [23] ValueComp ::= "eq" | "ne" | "lt" | "le" | "gt" | "ge"
// [22] GeneralComp ::= "=" | "!=" | "<" | "<=" | ">" | ">="
// [24] NodeComp ::= "is" | "<<" | ">>"
func parseComparisonExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "10 parseComparisonExpr")
	var lhs, rhs EvalFunc
	var err error
	if lhs, err = parseStringConcatExpr(tl); err != nil {
		leaveStep(tl, "10 parseComparisonExpr")
		return nil, err
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"=", "<", ">", "<=", ">=", "!=", "eq", "ne", "lt", "le", "gt", "ge"}); ok {
		if rhs, err = parseStringConcatExpr(tl); err != nil {
			leaveStep(tl, "10 parseComparisonExpr")
			return nil, err
		}
		leaveStep(tl, "10 parseComparisonExpr")
		return doCompare(op, lhs, rhs)
	}

	if op, ok := tl.readNexttokIfIsOneOfValue([]string{"is", "<<", ">>"}); ok {
		if rhs, err = parseStringConcatExpr(tl); err != nil {
			leaveStep(tl, "10 parseComparisonExpr")
			return nil, err
		}
		leaveStep(tl, "10 parseComparisonExpr")
		return doCompareNode(op, lhs, rhs)
	}

	leaveStep(tl, "10 parseComparisonExpr")
	return lhs, nil
}

// StringConcatExpr ::= RangeExpr ( "||" RangeExpr )*
func parseStringConcatExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "10a parseStringConcatExpr")
	var efs []EvalFunc
	for {
		ef, err := parseRangeExpr(tl)
		if err != nil {
			leaveStep(tl, "10a parseStringConcatExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if _, ok := tl.readNexttokIfIsOneOfValue([]string{"||"}); !ok {
			break
		}
	}

	if len(efs) == 1 {
		leaveStep(tl, "10a parseStringConcatExpr (#efs = 1)")
		return efs[0], nil
	}

	ef := func(ctx *Context) (Sequence, error) {
		var sb strings.Builder
		for _, ef := range efs {
			s, err := ef(ctx)
			if err != nil {
				return nil, err
			}
			sv, err := StringValue(s)
			if err != nil {
				return nil, err
			}
			sb.WriteString(sv)
		}
		return Sequence{sb.String()}, nil
	}

	leaveStep(tl, "10a parseStringConcatExpr")
	return ef, nil
}

// [11] RangeExpr ::= AdditiveExpr ( "to" AdditiveExpr )?
func parseRangeExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "11 parseRangeExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var err error
	for {
		ef, err = parseAdditiveExpr(tl)
		if err != nil {
			leaveStep(tl, "11 parseRangeExpr (err)")
			return nil, err
		}
		efs = append(efs, ef)
		if _, ok := tl.readNexttokIfIsOneOfValue([]string{"to"}); ok {
			// good, just add the next func to the efs slice
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "11 parseRangeExpr (#efs = 1)")
		return efs[0], nil
	}

	retf := func(ctx *Context) (Sequence, error) {
		lhs, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		rhs, err := efs[1](ctx)
		if err != nil {
			return nil, err
		}
		lhsNum, err := NumberValue(lhs)
		if err != nil {
			return nil, err
		}
		rhsNum, err := NumberValue(rhs)
		if err != nil {
			return nil, err
		}
		// Per XPath spec: if either is NaN or Inf, or lhs > rhs, return empty
		if math.IsNaN(lhsNum) || math.IsNaN(rhsNum) || math.IsInf(lhsNum, 0) || math.IsInf(rhsNum, 0) {
			return Sequence{}, nil
		}
		startF := math.Round(lhsNum)
		endF := math.Round(rhsNum)
		if startF > endF {
			return Sequence{}, nil
		}
		// Guard against extremely large ranges
		if endF-startF > 10_000_000 {
			return nil, fmt.Errorf("range too large: %v to %v", startF, endF)
		}
		start := int(startF)
		end := int(endF)
		// Guard against int overflow: if start or end is at MaxInt64,
		// the loop increment would overflow
		if start == math.MaxInt64 || end == math.MaxInt64 {
			if start == end {
				return Sequence{start}, nil
			}
			return nil, fmt.Errorf("range too large: integer overflow")
		}
		count := end - start + 1
		seq := make(Sequence, 0, count)
		for i := start; i <= end; i++ {
			seq = append(seq, i)
		}
		return seq, nil
	}
	leaveStep(tl, "11 parseRangeExpr")
	return retf, nil
}

// [12] AdditiveExpr ::= MultiplicativeExpr ( ("+" | "-") MultiplicativeExpr )*
func parseAdditiveExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "12 parseAdditiveExpr")
	var efs []EvalFunc
	var operator []string
	var ef EvalFunc
	var err error
	for {
		ef, err = parseMultiplicativeExpr(tl)
		if err != nil {
			leaveStep(tl, "12 parseAdditiveExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"+", "-"}); ok {
			operator = append(operator, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "12 parseAdditiveExpr")
		return efs[0], nil
	}
	ef = func(ctx *Context) (Sequence, error) {
		savedSeq := ctx.sequence
		s, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		if len(s) == 0 {
			return Sequence{}, nil
		}

		// Check if we're dealing with durations or date/time types
		result := s[0]
		for i := 1; i < len(efs); i++ {
			ctx.sequence = savedSeq
			s2, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			if len(s2) == 0 {
				return Sequence{}, nil
			}
			op := operator[i-1]
			res, err := addItems(result, s2[0], op)
			if err != nil {
				return nil, err
			}
			result = res
		}
		return Sequence{result}, nil
	}
	leaveStep(tl, "12 parseAdditiveExpr")
	return ef, nil
}

// [13] MultiplicativeExpr ::=  UnionExpr ( ("*" | "div" | "idiv" | "mod") UnionExpr )*
func parseMultiplicativeExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "13 parseMultiplicativeExpr")

	var efs []EvalFunc
	var operator []string
	var ef EvalFunc
	var err error
	for {
		ef, err = parseUnionExpr(tl)
		if err != nil {
			leaveStep(tl, "13 parseMultiplicativeExpr")
			return nil, err
		}
		if ef == nil {
			if len(efs) == 0 {
				return nil, nil
			}
			break
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValue([]string{"*", "div", "idiv", "mod"}); ok {
			operator = append(operator, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "13 parseMultiplicativeExpr")
		return efs[0], nil
	}

	ef = func(ctx *Context) (Sequence, error) {
		savedSeq := ctx.sequence
		s, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		if len(s) == 0 {
			return Sequence{}, nil
		}

		// Check for duration types in first operand
		if dur, ok := s[0].(XSDuration); ok {
			result := dur
			for i := 1; i < len(efs); i++ {
				ctx.sequence = savedSeq
				s2, err := efs[i](ctx)
				if err != nil {
					return nil, err
				}
				if len(s2) == 0 {
					return Sequence{}, nil
				}
				op := operator[i-1]
				// Duration * number, Duration div number, Duration div Duration
				if dur2, ok := s2[0].(XSDuration); ok && op == "div" {
					// Duration div Duration = number
					a := durationToSeconds(result)
					b := durationToSeconds(dur2)
					if b == 0 {
						return nil, NewXPathError("FODT0002", "division by zero-duration")
					}
					return Sequence{a / b}, nil
				}
				flt, err := NumberValue(Sequence{s2[0]})
				if err != nil {
					return nil, err
				}
				ms := durationToMonthsAndSeconds(result)
				switch op {
				case "*":
					ms.months = int(math.Round(float64(ms.months) * flt))
					ms.seconds = ms.seconds * flt
				case "div":
					if flt == 0 {
						return nil, NewXPathError("FODT0002", "division by zero")
					}
					ms.months = int(math.Round(float64(ms.months) / flt))
					ms.seconds = ms.seconds / flt
				}
				result = monthsAndSecondsToDuration(ms.months, ms.seconds)
			}
			return Sequence{result}, nil
		}

		// Check if second operand is duration (number * Duration)
		if len(efs) == 2 && operator[0] == "*" {
			ctx.sequence = savedSeq
			s2, err := efs[1](ctx)
			if err != nil {
				return nil, err
			}
			if len(s2) > 0 {
				if dur, ok := s2[0].(XSDuration); ok {
					flt, err := NumberValue(s)
					if err != nil {
						return nil, err
					}
					ms := durationToMonthsAndSeconds(dur)
					ms.months = int(math.Round(float64(ms.months) * flt))
					ms.seconds = ms.seconds * flt
					return Sequence{monthsAndSecondsToDuration(ms.months, ms.seconds)}, nil
				}
			}
		}

		sum, err := NumberValue(s)
		if err != nil {
			return nil, err
		}
		resultType := NumericType(s[0])
		for i := 1; i < len(efs); i++ {
			ctx.sequence = savedSeq
			s2, err := efs[i](ctx)
			if err != nil {
				return nil, err
			}
			if len(s2) == 0 {
				return Sequence{}, nil
			}
			flt, err := NumberValue(s2)
			opType := PromoteNumeric(resultType, NumericType(s2[0]))
			switch operator[i-1] {
			case "*":
				sum *= flt
				resultType = opType
			case "div":
				// div always produces at least decimal
				if opType < NumDecimal {
					opType = NumDecimal
				}
				// Division by zero raises FOAR0001 for integer/decimal operands.
				// For float/double, Go produces ±Inf which is correct per spec.
				if flt == 0 && opType <= NumDecimal {
					return nil, NewXPathError("FOAR0001", "division by zero")
				}
				sum /= flt
				resultType = opType
			case "idiv":
				if flt == 0 {
					return nil, NewXPathError("FOAR0002", "integer division by zero")
				}
				if math.IsNaN(sum) || math.IsInf(sum, 0) {
					return nil, NewXPathError("FOAR0002", "integer division with NaN or Inf")
				}
				sum = math.Trunc(sum / flt)
				resultType = NumInteger
			case "mod":
				sum = math.Mod(sum, flt)
				resultType = opType
			}
		}
		if resultType == NumInteger {
			return Sequence{int(sum)}, nil
		}
		return Sequence{WrapNumeric(sum, resultType)}, nil
	}

	leaveStep(tl, "13 parseMultiplicativeExpr")
	return ef, nil
}

// [14] UnionExpr ::= IntersectExceptExpr ( ("union" | "|") IntersectExceptExpr )*
func parseUnionExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "14 parseUnionExpr")
	var efs []EvalFunc

	for {
		ef, err := parseIntersectExceptExpr(tl)
		if err != nil {
			leaveStep(tl, "14 parseUnionExpr")
			return nil, err
		}
		efs = append(efs, ef)
		if _, found := tl.readNexttokIfIsOneOfValue([]string{"union", "|"}); !found {
			break
		}
	}

	if len(efs) == 1 {
		leaveStep(tl, "14 parseUnionExpr")
		return efs[0], nil
	}

	ret := func(ctx *Context) (Sequence, error) {
		if len(efs) == 1 {
			return efs[0](ctx)
		}
		// Save and restore ctx.sequence so each union branch
		// evaluates against the original context sequence.
		savedSeq := ctx.sequence
		var seq Sequence
		for _, ef := range efs {
			ctx.sequence = savedSeq
			efSeq, err := ef(ctx)
			if err != nil {
				return nil, err
			}
			seq = append(seq, efSeq...)
		}
		var nodes goxml.SortByDocumentOrder
		for _, itm := range seq {
			if n, ok := itm.(goxml.XMLNode); ok {
				nodes = append(nodes, n)
			}
		}
		// document order
		nodes = nodes.SortAndEliminateDuplicates()
		var retSeq Sequence
		for _, itm := range nodes {
			retSeq = append(retSeq, itm)
		}
		return retSeq, nil
	}

	leaveStep(tl, "14 parseUnionExpr")
	return ret, nil
}

// [15] IntersectExceptExpr  ::= InstanceofExpr ( ("intersect" | "except") InstanceofExpr )*
func parseIntersectExceptExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "15 parseIntersectExceptExpr")
	var efs []EvalFunc
	var intersectExcepts []string
	var err error
	var ef EvalFunc
	if ef, err = parseInstanceofExpr(tl); err != nil {
		leaveStep(tl, "15 parseIntersectExceptExpr")
		return nil, err
	}
	efs = append(efs, ef)
	for {
		var intersectExcept string
		var ok bool
		if intersectExcept, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"intersect", "except"}, tokQName); !ok {
			break
		}
		intersectExcepts = append(intersectExcepts, intersectExcept)
		if ef, err = parseInstanceofExpr(tl); err != nil {
			leaveStep(tl, "15 parseIntersectExceptExpr")
			return nil, err
		}
		efs = append(efs, ef)
	}
	if len(efs) == 1 {
		leaveStep(tl, "15 parseIntersectExceptExpr")
		return efs[0], nil

	}
	evaler := func(ctx *Context) (Sequence, error) {
		ret := Sequence{}
		var left Sequence
		for i, ef := range efs {
			newcontext := CopyContext(ctx)
			right, err := ef(newcontext)
			if err != nil {
				return nil, err
			}

			var lelt, relt *goxml.Element
			var ok, inRight bool
			if i > 0 {
				shouldBeInRight := intersectExcepts[i-1] == "intersect"
				ids := map[int]bool{}
				for _, rItem := range right {
					if relt, ok = rItem.(*goxml.Element); !ok {
						return nil, fmt.Errorf("FIXME: not an element")
					}
					ids[relt.ID] = true
				}
				for _, lItem := range left {
					if lelt, ok = lItem.(*goxml.Element); !ok {
						return nil, fmt.Errorf("FIXME: not an element")
					}
					if _, inRight = ids[lelt.ID]; inRight == shouldBeInRight {
						ret = append(ret, lelt)
					}
				}
			}
			left = right
		}
		ctx.sequence = ret
		return ret, nil
	}
	leaveStep(tl, "15 parseIntersectExceptExpr")
	return evaler, nil
}

// [16] InstanceofExpr ::= TreatExpr ( "instance" "of" SequenceType )?
func parseInstanceofExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "16 parseInstanceofExpr")
	var ef EvalFunc
	var tf testFunc
	var err error
	if ef, err = parseTreatExpr(tl); err != nil {
		leaveStep(tl, "16 parseInstanceofExpr")
		return nil, err
	}

	if tl.nexttokIsValue("instance") {
		tl.read()
		if !tl.nexttokIsValue("of") {
			tl.unread()
			leaveStep(tl, "16 parseInstanceofExpr")
			return ef, nil
		}
		tl.read()

		if tl.readIfTokenFollow([]token{{"empty-sequence", tokQName}, {'(', tokOpenParen}, {')', tokCloseParen}}) {
			evaler := func(ctx *Context) (Sequence, error) {
				seq, err := ef(ctx)
				if err != nil {
					return nil, err
				}
				return Sequence{len(seq) == 0}, nil
			}
			return evaler, nil
		}

		if tf, err = parseSequenceType(tl); err != nil {
			leaveStep(tl, "16 parseInstanceofExpr")
			return nil, err
		}
		var oi string
		oi, _ = tl.readNexttokIfIsOneOfValue([]string{"*", "+", "?"})
		inOfExpr := func(ctx *Context) (Sequence, error) {
			seq, err := ef(ctx)
			if err != nil {
				return nil, err
			}

			if oi == "" && len(seq) != 1 {
				return Sequence{false}, nil
			}
			if oi == "+" && len(seq) < 1 {
				return Sequence{false}, nil
			}
			if oi == "?" && len(seq) > 1 {
				return Sequence{false}, nil
			}

			if tf == nil {
				// No type test parsed — unknown type, always false
				return Sequence{false}, nil
			}
			for _, itm := range seq {
				if !tf(ctx, itm) {
					return Sequence{false}, nil
				}
			}

			return Sequence{true}, nil
		}
		leaveStep(tl, "16 parseInstanceofExpr")
		return inOfExpr, nil
	}

	leaveStep(tl, "16 parseInstanceofExpr")
	return ef, nil
}

// [17] TreatExpr ::= CastableExpr ( "treat" "as" SequenceType )?
func parseTreatExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "17 parseTreatExpr")
	var ef EvalFunc
	ef, err := parseCastableExpr(tl)
	if err != nil {
		leaveStep(tl, "17 parseTreatExpr")
		return nil, err
	}
	var ok bool
	if _, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"treat"}, tokQName); ok {
		if err = tl.skipNCName("as"); err != nil {
			leaveStep(tl, "17 parseTreatExpr")
			return nil, err
		}

		if tl.readIfTokenFollow([]token{{"empty-sequence", tokQName}, {'(', tokOpenParen}, {')', tokCloseParen}}) {
			evaler := func(ctx *Context) (Sequence, error) {
				seq, err := ef(ctx)
				if err != nil {
					return nil, err
				}
				return Sequence{len(seq) == 0}, nil
			}
			return evaler, nil
		}

		tf, err := parseSequenceType(tl)
		if err != nil {
			return nil, err
		}
		// Read optional occurrence indicator
		var oi string
		oi, _ = tl.readNexttokIfIsOneOfValue([]string{"*", "+", "?"})
		baseEf := ef
		ef = func(ctx *Context) (Sequence, error) {
			seq, err := baseEf(ctx)
			if err != nil {
				return nil, err
			}
			// Check cardinality
			switch oi {
			case "":
				if len(seq) != 1 {
					return nil, NewXPathError("XPDY0050", fmt.Sprintf("treat as requires exactly one item, got %d", len(seq)))
				}
			case "+":
				if len(seq) < 1 {
					return nil, NewXPathError("XPDY0050", "treat as requires at least one item, got empty")
				}
			case "?":
				if len(seq) > 1 {
					return nil, NewXPathError("XPDY0050", fmt.Sprintf("treat as requires at most one item, got %d", len(seq)))
				}
			}
			// Check type if tf is available
			if tf != nil {
				for _, itm := range seq {
					if !tf(ctx, itm) {
						return nil, NewXPathError("XPDY0050", "item does not match required type")
					}
				}
			}
			return seq, nil
		}
		leaveStep(tl, "17 parseTreatExpr")
		return ef, nil
	}

	leaveStep(tl, "17 parseTreatExpr")
	return ef, nil
}

// [18] CastableExpr ::= CastExpr ( "castable" "as" SingleType )?
func parseCastableExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "18 parseCastableExpr")
	var ef EvalFunc
	ef, err := parseCastExpr(tl)
	if err != nil {
		leaveStep(tl, "18 parseCastableExpr")
		return nil, err
	}

	var ok bool
	if _, ok = tl.readNexttokIfIsOneOfValueAndType([]string{"castable"}, tokQName); ok {
		if err = tl.skipNCName("as"); err != nil {
			leaveStep(tl, "18 parseCastableExpr")
			return nil, err
		}
		typTok, err := tl.read()
		if err != nil {
			leaveStep(tl, "18 parseCastableExpr")
			return nil, fmt.Errorf("expected type name after 'castable as'")
		}
		typName := typTok.Value.(string)
		// XPST0080: static error for abstract types
		switch typName {
		case "xs:NOTATION", "xs:anyAtomicType", "xs:anySimpleType":
			return nil, NewXPathError("XPST0080", fmt.Sprintf("cannot use %s as cast target", typName))
		}
		optional := false
		if _, optOk := tl.readNexttokIfIsOneOfValueAndType([]string{"?"}, tokOperator); optOk {
			optional = true
		}
		baseEf := ef
		ef = func(ctx *Context) (Sequence, error) {
			seq, err := baseEf(ctx)
			if err != nil {
				return nil, err
			}
			if len(seq) == 0 {
				return Sequence{optional}, nil
			}
			if len(seq) > 1 {
				return Sequence{false}, nil
			}
			item := seq[0]
			// Check type compatibility
			sourceType := TypeIDOf(item)
			if !castAllowed(sourceType, typName) {
				return Sequence{false}, nil
			}
			// XPST0080: cannot cast to abstract types
			switch typName {
			case "xs:NOTATION", "xs:anyAtomicType", "xs:anySimpleType":
				return Sequence{false}, nil
			}
			// Try the actual cast — if it succeeds, castable is true
			castPrefix := ""
			castLocal := typName
			if before, after, ok0 := strings.Cut(typName, ":"); ok0 {
				castPrefix = before
				castLocal = after
			}
			castNS := nsFN
			if castPrefix != "" {
				if ns, nsOk := ctx.Namespaces[castPrefix]; nsOk {
					castNS = ns
				}
			}
			fn := getfunction(castNS, castLocal)
			if fn != nil {
				_, castErr := fn.F(ctx, []Sequence{{item}})
				return Sequence{castErr == nil}, nil
			}
			return Sequence{false}, nil
		}
		leaveStep(tl, "18 parseCastableExpr")
		return ef, nil
	}

	leaveStep(tl, "18 parseCastableExpr")
	return ef, nil
}

// [19] CastExpr ::= ArrowExpr ( "cast" "as" SingleType )?
func parseCastExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "19 parseCastExpr")
	var ef EvalFunc
	ef, err := parseArrowExpr(tl)
	if err != nil {
		leaveStep(tl, "19 parseCastExpr")
		return nil, err
	}

	// Check for "cast as <type>"
	if tl.nexttokIsValue("cast") {
		tl.read() // consume "cast"
		if !tl.nexttokIsValue("as") {
			leaveStep(tl, "19 parseCastExpr")
			return nil, fmt.Errorf("expected 'as' after 'cast'")
		}
		tl.read() // consume "as"
		typTok, err := tl.read()
		if err != nil {
			leaveStep(tl, "19 parseCastExpr")
			return nil, fmt.Errorf("expected type name after 'cast as'")
		}
		typName := typTok.Value.(string)
		// XPST0080: static error for abstract types
		switch typName {
		case "xs:NOTATION", "xs:anyAtomicType", "xs:anySimpleType":
			return nil, NewXPathError("XPST0080", fmt.Sprintf("cannot use %s as cast target", typName))
		}
		// Check for optional "?" occurrence indicator.
		optional := false
		if _, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"?"}, tokOperator); ok {
			optional = true
		}
		baseEf := ef
		ef = func(ctx *Context) (Sequence, error) {
			seq, err := baseEf(ctx)
			if err != nil {
				return nil, err
			}
			if len(seq) == 0 {
				if optional {
					return Sequence{}, nil
				}
				return nil, NewXPathError("XPTY0004", fmt.Sprintf("empty sequence cannot be cast to %s", typName))
			}
			// Reject invalid cast target types
			switch typName {
			case "xs:NOTATION", "xs:anyAtomicType", "xs:anySimpleType":
				return nil, NewXPathError("XPST0080", fmt.Sprintf("cannot cast to %s", typName))
			}
			item := seq[0]
			// Validate cast compatibility
			sourceType := TypeIDOf(item)
			if !castAllowed(sourceType, typName) {
				return nil, NewXPathError("XPTY0004", fmt.Sprintf("cannot cast %s to %s", sourceType, typName))
			}
			switch typName {
			case "xs:integer", "xs:int",
				"xs:long", "xs:short", "xs:byte",
				"xs:unsignedLong", "xs:unsignedInt", "xs:unsignedShort", "xs:unsignedByte",
				"xs:nonPositiveInteger", "xs:nonNegativeInteger",
				"xs:negativeInteger", "xs:positiveInteger":
				// Route through registered constructor to get correct subtype tag
				castLocal := typName[3:] // strip "xs:"
				fn := getfunction(nsXS, castLocal)
				if fn != nil {
					return fn.F(ctx, []Sequence{{item}})
				}
				return xsInteger(ctx, []Sequence{{item}})
			case "xs:double":
				return xsDouble(ctx, []Sequence{{item}})
			case "xs:float":
				return xsFloat(ctx, []Sequence{{item}})
			case "xs:decimal":
				return xsDecimal(ctx, []Sequence{{item}})
			case "xs:string", "xs:normalizedString", "xs:token",
				"xs:language", "xs:NMTOKEN", "xs:Name", "xs:NCName",
				"xs:ID", "xs:IDREF", "xs:ENTITY",
				"xs:anyURI", "xs:untypedAtomic",
				"xs:hexBinary", "xs:base64Binary":
				// Route through registered constructor for correct type tag
				castLocal := typName[3:] // strip "xs:"
				fn := getfunction(nsXS, castLocal)
				if fn != nil {
					return fn.F(ctx, []Sequence{{item}})
				}
				sv, err := StringValue(Sequence{item})
				if err != nil {
					return nil, err
				}
				return Sequence{sv}, nil
			case "xs:boolean":
				// Handle numeric → boolean: 0/NaN → false, other → true
				if f, ok := ToFloat64(item); ok {
					return Sequence{f != 0 && !math.IsNaN(f)}, nil
				}
				if b, ok := item.(bool); ok {
					return Sequence{b}, nil
				}
				// xs:boolean cast uses XML Schema lexical rules
				sv, err := StringValue(Sequence{item})
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
			default:
				// Try calling the XSD constructor function
				castPrefix := ""
				castLocal := typName
				if before, after, ok := strings.Cut(typName, ":"); ok {
					castPrefix = before
					castLocal = after
				}
				castNS := nsFN
				if castPrefix != "" {
					if ns, ok := ctx.Namespaces[castPrefix]; ok {
						castNS = ns
					}
				}
				fn := getfunction(castNS, castLocal)
				if fn != nil {
					return fn.F(ctx, []Sequence{{item}})
				}
				return nil, NewXPathError("XPTY0004", fmt.Sprintf("unsupported cast type %s", typName))
			}
		}
	}

	leaveStep(tl, "19 parseCastExpr")
	return ef, nil
}

// [29] ArrowExpr ::= UnaryExpr ("=>" ArrowFunctionSpecifier ArgumentList)*
func parseArrowExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "29 parseArrowExpr")
	ef, err := parseUnaryExpr(tl)
	if err != nil {
		leaveStep(tl, "29 parseArrowExpr")
		return nil, err
	}

	for {
		if _, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"=>"}, tokOperator); !ok {
			break
		}
		// ArrowFunctionSpecifier ::= EQName | VarRef | ParenthesizedExpr
		fnTok, err := tl.read()
		if err != nil {
			leaveStep(tl, "29 parseArrowExpr")
			return nil, fmt.Errorf("expected function name after '=>'")
		}
		var fnName string
		if fnTok.Typ == tokQName {
			fnName = fnTok.Value.(string)
		} else {
			leaveStep(tl, "29 parseArrowExpr")
			return nil, fmt.Errorf("expected function name after '=>', got %v", fnTok)
		}

		// Parse ArgumentList: "(" (ExprSingle ("," ExprSingle)*)? ")"
		if err := tl.skipType(tokOpenParen); err != nil {
			leaveStep(tl, "29 parseArrowExpr")
			return nil, fmt.Errorf("'(' expected after arrow function name")
		}
		var argEfs []EvalFunc
		if !tl.nexttokIsTyp(tokCloseParen) {
			for {
				argEf, err := parseExprSingle(tl)
				if err != nil {
					return nil, err
				}
				argEfs = append(argEfs, argEf)
				if !tl.nexttokIsTyp(tokComma) {
					break
				}
				tl.read() // consume comma
			}
		}
		if err := tl.skipType(tokCloseParen); err != nil {
			return nil, fmt.Errorf("')' expected in arrow function arguments")
		}

		baseEf := ef
		capturedName := fnName
		capturedArgEfs := argEfs
		ef = func(ctx *Context) (Sequence, error) {
			// Evaluate the left-hand side (becomes first argument)
			leftSeq, err := baseEf(ctx)
			if err != nil {
				return nil, err
			}
			// Evaluate additional arguments
			allArgs := make([]Sequence, 1+len(capturedArgEfs))
			allArgs[0] = leftSeq
			for i, argEf := range capturedArgEfs {
				argSeq, err := argEf(ctx)
				if err != nil {
					return nil, err
				}
				allArgs[i+1] = argSeq
			}
			// Resolve function name and call
			fnPrefix := ""
			fnLocalName := capturedName
			if before, after, ok := strings.Cut(capturedName, ":"); ok {
				fnPrefix = before
				fnLocalName = after
			}
			return callFunctionResolved(fnPrefix, fnLocalName, allArgs, ctx)
		}
	}

	leaveStep(tl, "29 parseArrowExpr")
	return ef, nil
}

// [20] UnaryExpr ::= ("-" | "+")* ValueExpr
func parseUnaryExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "20 parseUnaryExpr")
	var hasOP bool
	mult := 1
	for {
		if op, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"+", "-"}, tokOperator); ok {
			hasOP = true
			if op == "-" {
				mult *= -1
			}
		} else {
			break
		}
	}
	pv, err := parseSimpleMapExpr(tl)
	if err != nil {
		leaveStep(tl, "20 parseUnaryExpr")
		return nil, err
	}

	if !hasOP {
		leaveStep(tl, "20 parseUnaryExpr")
		return pv, nil
	}
	var ef EvalFunc
	ef = func(ctx *Context) (Sequence, error) {
		if mult == -1 {
			seq, err := pv(ctx)
			if err != nil {
				return nil, err
			}
			flt, err := NumberValue(seq)
			if err != nil {
				return nil, err
			}
			return Sequence{flt * -1}, nil
		}
		return pv(ctx)
	}

	leaveStep(tl, "20 parseUnaryExpr")
	return ef, nil
}

// [21] SimpleMapExpr ::= PathExpr ("!" PathExpr)*
func parseSimpleMapExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "21 parseSimpleMapExpr")
	var efs []EvalFunc
	ef, err := parsePathExpr(tl)
	if err != nil {
		leaveStep(tl, "21 parseSimpleMapExpr")
		return nil, err
	}
	if ef == nil {
		leaveStep(tl, "21 parseSimpleMapExpr (nil)")
		return nil, nil
	}
	efs = append(efs, ef)
	for {
		if _, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"!"}, tokOperator); !ok {
			break
		}
		ef2, err := parsePathExpr(tl)
		if err != nil {
			leaveStep(tl, "21 parseSimpleMapExpr")
			return nil, err
		}
		if ef2 == nil {
			return nil, fmt.Errorf("expected expression after '!'")
		}
		efs = append(efs, ef2)
	}
	if len(efs) == 1 {
		leaveStep(tl, "21 parseSimpleMapExpr")
		return efs[0], nil
	}
	mapEf := func(ctx *Context) (Sequence, error) {
		result, err := efs[0](ctx)
		if err != nil {
			return nil, err
		}
		for _, stepEf := range efs[1:] {
			var newResult Sequence
			saveSeq := ctx.SetContextSequence(result)
			savePos := ctx.Pos
			saveSize := ctx.Size()
			ctx.SetSize(len(result))
			for pos, item := range result {
				ctx.Pos = pos
				ctx.SetContextSequence(Sequence{item})
				seq, err := stepEf(ctx)
				if err != nil {
					ctx.SetContextSequence(saveSeq)
					ctx.Pos = savePos
					ctx.SetSize(saveSize)
					return nil, err
				}
				newResult = append(newResult, seq...)
			}
			ctx.SetContextSequence(saveSeq)
			ctx.Pos = savePos
			ctx.SetSize(saveSize)
			result = newResult
		}
		return result, nil
	}
	leaveStep(tl, "21 parseSimpleMapExpr")
	return mapEf, nil
}

// [25] PathExpr ::= ("/" RelativePathExpr?) | ("//" RelativePathExpr) | RelativePathExpr
func parsePathExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "25 parsePathExpr")
	var rpe EvalFunc
	var op string
	var hasOP bool
	op, hasOP = tl.readNexttokIfIsOneOfValueAndType([]string{"/", "//"}, tokOperator)

	rpe, err := parseRelativePathExpr(tl)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// EOF is not an error
			leaveStep(tl, "25 parsePathExpr (EOF)")
			return func(ctx *Context) (Sequence, error) {
				return Sequence{ctx.Document()}, nil
			}, nil
		}
		leaveStep(tl, "25 parsePathExpr (err)")
		return nil, err
	}

	if hasOP {
		fn := func(ctx *Context) (Sequence, error) {
			ctx.Document()
			if op == "//" {
				ctx.descendantOrSelfAxis(isNode)
			}
			if rpe == nil {
				if op == "/" {
					return Sequence{ctx.Document()}, nil
				}
				return nil, fmt.Errorf("unexpected end of path expression after '//'")
			}
			seq, err := rpe(ctx)
			if err != nil {
				return nil, err
			}
			// For "//" paths, sort result in document order and
			// eliminate duplicates (XPath spec §3.3.2).
			// Only sort when all items are XMLNodes with unique non-zero
			// IDs (elements/documents from parsing have those;
			// attributes created on the fly have ID=0).
			if op == "//" && len(seq) > 1 {
				var nodes goxml.SortByDocumentOrder
				canSort := true
				for _, itm := range seq {
					n, ok := itm.(goxml.XMLNode)
					if !ok || n.GetID() == 0 {
						canSort = false
						break
					}
					nodes = append(nodes, n)
				}
				if canSort {
					nodes = nodes.SortAndEliminateDuplicates()
					seq = make(Sequence, len(nodes))
					for i, n := range nodes {
						seq[i] = n
					}
				}
			}
			return seq, nil
		}
		return fn, nil
	}

	leaveStep(tl, "25 parsePathExpr")
	return rpe, nil
}

// [26] RelativePathExpr ::= StepExpr (("/" | "//") StepExpr)*
func parseRelativePathExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "26 parseRelativePathExpr")
	var ef EvalFunc
	var efs []EvalFunc
	var ops []string

	for {
		ef, err := parseStepExpr(tl)
		if err != nil {
			leaveStep(tl, "26 parseRelativePathExpr (err)")
			return nil, err
		}
		if ef == nil {
			if len(efs) == 0 {
				return nil, nil
			}
			break
		}
		efs = append(efs, ef)
		if op, ok := tl.readNexttokIfIsOneOfValueAndType([]string{"/", "//"}, tokOperator); ok {
			ops = append(ops, op)
		} else {
			break
		}
	}
	if len(efs) == 1 {
		leaveStep(tl, "26 parseRelativePathExpr (1)")
		return efs[0], nil // just a simple StepExpr
	}

	ef = func(ctx *Context) (Sequence, error) {
		var retseq Sequence
		var seq Sequence
		var err error
		for i := 0; i < len(efs); i++ {
			ef := efs[i]
			retseq = retseq[:0]
			if len(ctx.sequence) == 0 {
				if seq, err = ef(ctx); err != nil {
					return nil, err
				}
				retseq = append(retseq, seq...)
			} else {
				copyContext := ctx.sequence
				ctx.size = len(copyContext)
				for j, itm := range copyContext {
					ctx.sequence = Sequence{itm}
					ctx.Pos = j + 1
					if seq, err = ef(ctx); err != nil {
						return nil, err
					}
					retseq = append(retseq, seq...)
				}
			}
			ctx.sequence = ctx.sequence[:0]
			for _, itm := range retseq {
				ctx.sequence = append(ctx.sequence, itm)
			}

			if i < len(ops) && ops[i] == "//" {
				ctx.descendantOrSelfAxis(isElement)
				retseq = append(retseq, ctx.sequence...)
			}
		}

		return retseq, nil
	}

	leaveStep(tl, "26 parseRelativePathExpr")
	return ef, nil
}

// [27] StepExpr := FilterExpr | AxisStep
func parseStepExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "27 parseStepExpr")
	var ef EvalFunc
	ef, err := parseFilterExpr(tl)
	if err != nil {
		leaveStep(tl, "27 parseStepExpr (err1)")
		return nil, err
	}
	if ef == nil {
		ef, err = parseAxisStep(tl)
	}
	if err != nil {
		leaveStep(tl, "27 parseStepExpr (err)")
		return nil, err
	}

	if ef == nil {
		return nil, nil
	}
	leaveStep(tl, "27 parseStepExpr")
	return ef, nil
}

// [28] AxisStep ::= (ReverseStep | ForwardStep) PredicateList
// [39] PredicateList ::= Predicate*
func parseAxisStep(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "28 parseAxisStep")
	var ef EvalFunc
	var err error
	var predicates []EvalFunc
	if ef, err = parseForwardStep(tl); err != nil {
		leaveStep(tl, "28 parseAxisStep (err)")
		return nil, err
	}
	for {
		if tl.nexttokIsTyp(tokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				leaveStep(tl, "28 parseAxisStep (err)")
				return nil, err
			}
			predicates = append(predicates, predicate)
			err = tl.skipType(tokCloseBracket)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	if len(predicates) == 0 {
		leaveStep(tl, "28 parseAxisStep (b)")
		return ef, nil
	}
	ff := func(ctx *Context) (Sequence, error) {
		var err error
		_, err = ef(ctx)
		if err != nil {
			return nil, err
		}
		for _, predicate := range predicates {
			_, err = ctx.Filter(predicate)
			if err != nil {
				return nil, err
			}
			ctx.size = len(ctx.sequence)
		}
		return ctx.sequence, nil
	}

	leaveStep(tl, "28 parseAxisStep (b)")
	return ff, nil
}

type axis int

const (
	axisChild axis = iota
	axisSelf
	axisDescendant
	axisDescendantOrSelf
	axisFollowing
	axisFollowingSibling
	axisParent
	axisAncestor
	axisAncestorOrSelf
	axisPreceding
	axisPrecedingSibling
)

func (a axis) String() string {
	switch a {
	case axisChild:
		return "child"
	case axisSelf:
		return "self"
	case axisDescendant:
		return "descendant"
	case axisDescendantOrSelf:
		return "descendant-or-self"
	case axisFollowing:
		return "following"
	case axisFollowingSibling:
		return "following-sibling"
	case axisParent:
		return "parent"
	case axisAncestor:
		return "ancestor"
	case axisAncestorOrSelf:
		return "ancestor-or-self"
	case axisPreceding:
		return "preceding"
	case axisPrecedingSibling:
		return "preceding-sibling"

	}
	return ""
}

// [29] ForwardStep ::= (ForwardAxis NodeTest) | AbbrevForwardStep
// [31] AbbrevForwardStep ::= "@"? NodeTest
func parseForwardStep(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "29 parseForwardStep")
	var err error

	stepAxis := axisChild
	tl.attributeMode = false

	if tl.nexttokIsTyp(tokDoubleColon) {
		nexttok, err := tl.read()
		if err != nil {
			return nil, err
		}

		switch nexttok.Value.(string) {
		case "attribute":
			tl.attributeMode = true
		case "child":
			stepAxis = axisChild
		case "self":
			stepAxis = axisSelf
		case "descendant":
			stepAxis = axisDescendant
		case "descendant-or-self":
			stepAxis = axisDescendantOrSelf
		case "following":
			stepAxis = axisFollowing
		case "following-sibling":
			stepAxis = axisFollowingSibling
		case "parent":
			stepAxis = axisParent
		case "ancestor":
			stepAxis = axisAncestor
		case "ancestor-or-self":
			stepAxis = axisAncestorOrSelf
		case "preceding-sibling":
			stepAxis = axisPrecedingSibling
		case "preceding":
			stepAxis = axisPreceding
		default:
			return nil, fmt.Errorf("unknown axis %s", nexttok.Value.(string))
		}
	}
	if tl.nexttokIsValue("..") && tl.nexttokIsTyp(tokOperator) {
		tl.read()
		ef := func(ctx *Context) (Sequence, error) {
			return ctx.parentAxis(isNode)
		}
		leaveStep(tl, "29 parseForwardStep (..)")
		return ef, nil
	}

	if tl.nexttokIsValue("@") {
		tl.read() // @
		tl.attributeMode = true
	}
	var tf testFunc
	if tf, err = parseNodeTest(tl); err != nil {
		leaveStep(tl, "29 parseForwardStep (err)")
		return nil, err
	}
	if tf == nil {
		leaveStep(tl, "29 parseForwardStep (nil)")
		return nil, nil
	}
	ret := func(ctx *Context) (Sequence, error) {
		var ret Sequence
		var err error
		switch stepAxis {
		case axisSelf:
			// nothing
		case axisChild:
			_, err = ctx.childAxis(tf)
		case axisDescendant:
			_, err = ctx.descendantAxis(tf)
		case axisDescendantOrSelf:
			_, err = ctx.descendantOrSelfAxis(tf)
		case axisFollowing:
			_, err = ctx.followingAxis(tf)
		case axisFollowingSibling:
			_, err = ctx.followingSiblingAxis(tf)
		case axisParent:
			_, err = ctx.parentAxis(tf)
		case axisAncestor:
			_, err = ctx.ancestorAxis(tf)
		case axisAncestorOrSelf:
			_, err = ctx.ancestorOrSelfAxis(tf)
		case axisPrecedingSibling:
			_, err = ctx.precedingSiblingAxis(tf)
		case axisPreceding:
			_, err = ctx.precedingAxis(tf)
		default:
			return nil, fmt.Errorf("unknown axis %s", stepAxis)
		}
		if err != nil {
			return nil, err
		}
		copyContext := ctx.sequence
		for _, itm := range copyContext {
			ret = append(ret, itm)
		}
		ctx.sequence = ret
		ctx.size = len(ret)
		return ret, nil
	}

	leaveStep(tl, "29 parseForwardStep")
	return ret, nil
}

// [30] ForwardAxis ::= ("child" "::") | ("descendant" "::")| ("attribute" "::")| ("self" "::")| ("descendant-or-self" "::")| ("following-sibling" "::")| ("following" "::")| ("namespace" "::")
// [32] ReverseStep ::= (ReverseAxis NodeTest) | AbbrevReverseStep
// [34] AbbrevReverseStep ::= ".."
// [33] ReverseAxis ::= ("parent" "::") | ("ancestor" "::") | ("preceding-sibling" "::") | ("preceding" "::") | ("ancestor-or-self" "::")
// [35] NodeTest ::= KindTest | NameTest
func parseNodeTest(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "35 parseNodeTest")
	var tf testFunc
	var err error
	if str, found := tl.readNexttokIfIsOneOfValueAndType(kindTestStrings, tokQName); found {
		if !tl.nexttokIsTyp(tokOpenParen) {
			tl.unread() // unread the kindTest name (e.g. "text")
		} else if err = tl.skipType(tokOpenParen); err != nil {
			tl.unread()
		} else {
			tl.unread()
			if tf, err = parseKindTest(tl, str); err != nil {
				return nil, err
			}
			if tf != nil {
				return tf, nil
			}
		}
	}
	if tf, err = parseNameTest(tl); err != nil {
		leaveStep(tl, "35 parseNodeTest (err)")
		return nil, err
	}

	leaveStep(tl, "35 parseNodeTest")
	return tf, nil
}

// [36] NameTest ::= QName | Wildcard
func parseNameTest(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "36 parseNameTest")
	var tf testFunc

	if tl.nexttokIsTyp(tokQName) {
		n, err := tl.read()
		if err != nil {
			leaveStep(tl, "36 parseNameTest (err)")
			return nil, err
		}
		var name string
		var ok bool
		if name, ok = n.Value.(string); !ok {
			return nil, err
		}
		if tl.attributeMode {
			tf = returnAttributeNameTest(name)
		} else {
			tf = returnElementNameTest(name)
		}

		leaveStep(tl, "36 parseNameTest")
		return tf, nil
	}
	var err error
	tf, err = parseWildCard(tl)
	if err != nil {
		leaveStep(tl, "36 parseNameTest (err)")
		return nil, err
	}
	leaveStep(tl, "36 parseNameTest")
	return tf, nil
}

// [37] Wildcard ::= "*" | (NCName ":" "*") | ("*" ":" NCName)
func parseWildCard(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "37 parseWildCard")
	var tf testFunc
	var err error
	var strTok *token
	if strTok, err = tl.read(); err != nil {
		leaveStep(tl, "37 parseWildCard (err)")
		return nil, err
	}

	if str, ok := strTok.Value.(string); ok {
		if str == "*" || strings.HasPrefix(str, "*:") || strings.HasSuffix(str, ":*") {
			if tl.attributeMode {
				tf = func(ctx *Context, itm Item) bool {
					if _, ok := itm.(*goxml.Attribute); ok {
						return true
					}
					return false
				}
			} else {
				tf = func(ctx *Context, itm Item) bool {
					if _, ok := itm.(*goxml.Element); ok {
						return true
					}
					return false
				}
			}
		} else {
			tl.unread()
		}
	} else {
		tl.unread()
	}
	leaveStep(tl, "37 parseWildCard")
	return tf, nil
}

// [38] FilterExpr ::= PrimaryExpr PredicateList
// [39] PredicateList ::= Predicate*
// [40] Predicate ::= "[" Expr "]"
func parseFilterExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "38 parseFilterExpr")

	var ef EvalFunc
	ef, err := parsePrimaryExpr(tl)
	if err != nil {
		leaveStep(tl, "38 parseFilterExpr (err)")
		return nil, err
	}

	// PostfixExpr ::= PrimaryExpr (Predicate | Lookup)*
	modified := false
	for {
		if tl.nexttokIsTyp(tokOpenBracket) {
			tl.read()
			predicate, err := parseExpr(tl)
			if err != nil {
				return nil, err
			}
			if err = tl.skipType(tokCloseBracket); err != nil {
				return nil, err
			}
			baseEf := ef
			ef = func(ctx *Context) (Sequence, error) {
				ctx.sequence, err = baseEf(ctx)
				if err != nil {
					return nil, err
				}
				_, err = ctx.Filter(predicate)
				if err != nil {
					return nil, err
				}
				ctx.ctxPositions = nil
				ctx.ctxLengths = nil
				return ctx.sequence, nil
			}
			modified = true
		} else if tl.nexttokIsValue("?") {
			tl.read() // consume ?
			lookupEf, err := parseLookupKeySpecifier(tl)
			if err != nil {
				return nil, err
			}
			baseEf := ef
			ef = func(ctx *Context) (Sequence, error) {
				base, err := baseEf(ctx)
				if err != nil {
					return nil, err
				}
				return evalLookup(ctx, base, lookupEf)
			}
			modified = true
		} else if tl.nexttokIsTyp(tokOpenParen) {
			// Dynamic function call: expr(args)
			tl.read() // consume (
			var argEfs []EvalFunc
			if !tl.nexttokIsTyp(tokCloseParen) {
				for {
					argEf, err := parseExprSingle(tl)
					if err != nil {
						return nil, err
					}
					argEfs = append(argEfs, argEf)
					if !tl.nexttokIsTyp(tokComma) {
						break
					}
					tl.read() // consume comma
				}
			}
			if err := tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			baseEf := ef
			capturedArgEfs := argEfs
			ef = func(ctx *Context) (Sequence, error) {
				base, err := baseEf(ctx)
				if err != nil {
					return nil, err
				}
				if len(base) != 1 {
					return nil, NewXPathError("XPTY0004", "dynamic function call requires single function item")
				}
				fn, ok := base[0].(*XPathFunction)
				if !ok {
					// Could be a map or array lookup
					if m, ok := base[0].(*XPathMap); ok && len(capturedArgEfs) == 1 {
						keySeq, err := capturedArgEfs[0](ctx)
						if err != nil {
							return nil, err
						}
						if len(keySeq) > 0 {
							val, _ := m.Get(keySeq[0])
							return val, nil
						}
						return Sequence{}, nil
					}
					if arr, ok := base[0].(*XPathArray); ok && len(capturedArgEfs) == 1 {
						idxSeq, err := capturedArgEfs[0](ctx)
						if err != nil {
							return nil, err
						}
						idx, err := NumberValue(idxSeq)
						if err != nil {
							return nil, err
						}
						return arr.Get(int(idx))
					}
					return nil, NewXPathError("XPTY0004", fmt.Sprintf("cannot call %T as function", base[0]))
				}
				args := make([]Sequence, len(capturedArgEfs))
				for i, argEf := range capturedArgEfs {
					args[i], err = argEf(ctx)
					if err != nil {
						return nil, err
					}
				}
				return fn.Call(ctx, args)
			}
			modified = true
		} else {
			break
		}
	}
	_ = modified
	leaveStep(tl, "38 parseFilterExpr")
	return ef, nil
}

// parseLookupKeySpecifier parses: KeySpecifier ::= NCName | IntegerLiteral | ParenthesizedExpr | "*"
// Returns an EvalFunc that produces the key(s) to look up, or nil for wildcard (*).
type lookupSpec struct {
	wildcard bool
	ef       EvalFunc
}

func parseLookupKeySpecifier(tl *Tokenlist) (*lookupSpec, error) {
	// Wildcard: ?*
	if tl.nexttokIsValue("*") {
		tl.read()
		return &lookupSpec{wildcard: true}, nil
	}
	// ParenthesizedExpr: ?(expr)
	if tl.nexttokIsTyp(tokOpenParen) {
		tl.read()
		ef, err := parseExpr(tl)
		if err != nil {
			return nil, err
		}
		if err := tl.skipType(tokCloseParen); err != nil {
			return nil, fmt.Errorf("')' expected in lookup expression")
		}
		return &lookupSpec{ef: ef}, nil
	}
	// IntegerLiteral
	if tl.nexttokIsTyp(tokNumber) {
		tok, _ := tl.read()
		numVal := tok.Value
		return &lookupSpec{ef: func(ctx *Context) (Sequence, error) {
			return Sequence{numVal}, nil
		}}, nil
	}
	// NCName
	if tl.nexttokIsTyp(tokQName) {
		tok, _ := tl.read()
		name := tok.Value.(string)
		return &lookupSpec{ef: func(ctx *Context) (Sequence, error) {
			return Sequence{name}, nil
		}}, nil
	}
	return nil, fmt.Errorf("expected key specifier after '?'")
}

// evalLookup applies a lookup operation to each item in the base sequence.
func evalLookup(ctx *Context, base Sequence, spec *lookupSpec) (Sequence, error) {
	var result Sequence
	for _, item := range base {
		switch v := item.(type) {
		case *XPathMap:
			if spec.wildcard {
				for _, entry := range v.Entries {
					result = append(result, entry.Value...)
				}
			} else {
				keys, err := spec.ef(ctx)
				if err != nil {
					return nil, err
				}
				for _, key := range keys {
					if val, ok := v.Get(key); ok {
						result = append(result, val...)
					}
				}
			}
		case *XPathArray:
			if spec.wildcard {
				for _, member := range v.Members {
					result = append(result, member...)
				}
			} else {
				keys, err := spec.ef(ctx)
				if err != nil {
					return nil, err
				}
				for _, key := range keys {
					idx, err := NumberValue(Sequence{key})
					if err != nil {
						return nil, err
					}
					member, err := v.Get(int(idx))
					if err != nil {
						return nil, err
					}
					result = append(result, member...)
				}
			}
		default:
			return nil, NewXPathError("XPTY0004", fmt.Sprintf("lookup operator requires a map or array, got %T", item))
		}
	}
	return result, nil
}

// [41] PrimaryExpr ::= Literal | VarRef | ParenthesizedExpr | ContextItemExpr | FunctionCall
func parsePrimaryExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "41 parsePrimaryExpr")
	var ef EvalFunc

	nexttok, err := tl.read()
	if err != nil {
		leaveStep(tl, "41 parsePrimaryExpr (err) ")
		return nil, err
	}

	// StringLiteral
	if nexttok.Typ == tokString {
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{nexttok.Value.(string)}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// NumericLiteral
	if nexttok.Typ == tokNumber {
		numVal := nexttok.Value // int, XSDecimal, or XSDouble
		ef = func(ctx *Context) (Sequence, error) {
			return Sequence{numVal}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// ParenthesizedExpr
	if nexttok.Typ == tokOpenParen {
		ef, err = parseParenthesizedExpr(tl)
		if err != nil {
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// VarRef — possibly followed by "(" for map/array/function-item call
	if nexttok.Typ == tokVarname {
		varname := nexttok.Value.(string)
		if tl.nexttokIsTyp(tokOpenParen) {
			// $var(args) — dynamic function call / map lookup / array lookup
			tl.read() // consume (
			var argEfs []EvalFunc
			if !tl.nexttokIsTyp(tokCloseParen) {
				for {
					aef, err := parseExprSingle(tl)
					if err != nil {
						return nil, err
					}
					argEfs = append(argEfs, aef)
					if !tl.nexttokIsTyp(tokComma) {
						break
					}
					tl.read() // consume comma
				}
			}
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, fmt.Errorf("close paren expected after $%s(...)", varname)
			}
			ef = func(ctx *Context) (Sequence, error) {
				varVal := ctx.vars[varname]
				// Evaluate arguments
				args := make([]Sequence, len(argEfs))
				for i, aef := range argEfs {
					seq, err := aef(ctx)
					if err != nil {
						return nil, err
					}
					args[i] = seq
				}
				if len(varVal) == 1 {
					switch v := varVal[0].(type) {
					case *XPathMap:
						if len(args) == 1 && len(args[0]) > 0 {
							val, _ := v.Get(args[0][0])
							return val, nil
						}
					case *XPathArray:
						if len(args) == 1 {
							idx, err := NumberValue(args[0])
							if err != nil {
								return nil, err
							}
							return v.Get(int(idx))
						}
					case *XPathFunction:
						return v.Call(ctx, args)
					}
				}
				return nil, fmt.Errorf("$%s is not a callable function, map, or array", varname)
			}
			leaveStep(tl, "41 parsePrimaryExpr (var-call)")
			return ef, nil
		}
		ef = func(ctx *Context) (Sequence, error) {
			return ctx.vars[varname], nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// Unary Lookup: ?key (operates on context item)
	if nexttok.Typ == tokOperator && nexttok.Value.(string) == "?" {
		spec, err := parseLookupKeySpecifier(tl)
		if err != nil {
			return nil, err
		}
		ef = func(ctx *Context) (Sequence, error) {
			return evalLookup(ctx, ctx.sequence, spec)
		}
		leaveStep(tl, "41 parsePrimaryExpr (unary-lookup)")
		return ef, nil
	}

	// Context item
	if nexttok.Typ == tokOperator && nexttok.Value.(string) == "." {
		ef = func(ctx *Context) (Sequence, error) {
			return ctx.sequence, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr")
		return ef, nil
	}

	// InlineFunctionExpr: function($x, $y) { expr }
	if nexttok.Typ == tokQName && nexttok.Value.(string) == "function" && tl.nexttokIsTyp(tokOpenParen) {
		tl.read() // consume (
		var paramNames []string
		if !tl.nexttokIsTyp(tokCloseParen) {
			for {
				pTok, err := tl.read()
				if err != nil || pTok.Typ != tokVarname {
					return nil, fmt.Errorf("expected parameter name in inline function")
				}
				paramNames = append(paramNames, pTok.Value.(string))
				// Skip optional "as SequenceType"
				if tl.nexttokIsValue("as") {
					tl.read() // consume "as"
					// Skip type tokens until we see , or )
					for {
						if tl.nexttokIsTyp(tokComma) || tl.nexttokIsTyp(tokCloseParen) {
							break
						}
						if _, err := tl.read(); err != nil {
							return nil, err
						}
					}
				}
				if !tl.nexttokIsTyp(tokComma) {
					break
				}
				tl.read() // consume comma
			}
		}
		if err := tl.skipType(tokCloseParen); err != nil {
			return nil, fmt.Errorf("')' expected in inline function parameter list")
		}
		// Skip optional "as SequenceType"
		if tl.nexttokIsValue("as") {
			tl.read() // consume "as"
			for {
				if tl.nexttokIsTyp(tokOpenBrace) {
					break
				}
				if _, err := tl.read(); err != nil {
					return nil, err
				}
			}
		}
		// Parse function body: { Expr }
		if err := tl.skipType(tokOpenBrace); err != nil {
			return nil, fmt.Errorf("'{' expected in inline function body")
		}
		bodyEf, err := parseExpr(tl)
		if err != nil {
			return nil, err
		}
		if err := tl.skipType(tokCloseBrace); err != nil {
			return nil, fmt.Errorf("'}' expected in inline function body")
		}
		capturedParams := paramNames
		ef = func(ctx *Context) (Sequence, error) {
			// Capture current variable scope for closure
			closureVars := make(map[string]Sequence, len(ctx.vars))
			maps.Copy(closureVars, ctx.vars)
			fnRef := &XPathFunction{
				Name:  "(anonymous)",
				Arity: len(capturedParams),
				Fn: func(callCtx *Context, args []Sequence) (Sequence, error) {
					// Save current vars
					savedVars := make(map[string]Sequence, len(callCtx.vars))
					maps.Copy(savedVars, callCtx.vars)
					// Apply closure vars
					maps.Copy(callCtx.vars, closureVars)
					// Bind parameters
					for i, name := range capturedParams {
						if i < len(args) {
							callCtx.vars[name] = args[i]
						}
					}
					result, err := bodyEf(callCtx)
					// Restore vars
					clear(callCtx.vars)
					maps.Copy(callCtx.vars, savedVars)
					return result, err
				},
			}
			return Sequence{fnRef}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr (inline-func)")
		return ef, nil
	}

	// Map constructor: map { key: value, ... }
	if nexttok.Typ == tokQName && nexttok.Value.(string) == "map" && tl.nexttokIsTyp(tokOpenBrace) {
		tl.read() // consume {
		ef, err = parseMapConstructor(tl)
		if err != nil {
			leaveStep(tl, "41 parsePrimaryExpr (err map)")
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr (map)")
		return ef, nil
	}

	// Square array constructor: [ expr, expr, ... ]
	if nexttok.Typ == tokOpenBracket {
		ef, err = parseSquareArrayConstructor(tl)
		if err != nil {
			leaveStep(tl, "41 parsePrimaryExpr (err square-array)")
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr (square-array)")
		return ef, nil
	}

	// Array constructor: array { expr, expr, ... }
	if nexttok.Typ == tokQName && nexttok.Value.(string) == "array" && tl.nexttokIsTyp(tokOpenBrace) {
		tl.read() // consume {
		ef, err = parseArrayConstructor(tl)
		if err != nil {
			leaveStep(tl, "41 parsePrimaryExpr (err array)")
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr (array)")
		return ef, nil
	}

	// NamedFunctionRef: EQName "#" IntegerLiteral
	if (nexttok.Typ == tokQName || nexttok.Typ == tokEQName) && tl.nexttokIsValue("#") {
		fnFullName := nexttok.Value.(string)
		tl.read() // consume #
		arityTok, err := tl.read()
		if err != nil {
			return nil, fmt.Errorf("expected arity after '#' in function reference")
		}
		arityF, _ := ToFloat64(arityTok.Value)
		arity := int(arityF)
		var capturedNS, capturedLocal, capturedPrefix string
		if nexttok.Typ == tokEQName {
			// URIQualifiedName: "namespace}localname"
			if before, after, ok := strings.Cut(fnFullName, "}"); ok {
				capturedNS = before
				capturedLocal = after
			}
		} else {
			capturedLocal = fnFullName
			if before, after, ok := strings.Cut(fnFullName, ":"); ok {
				capturedPrefix = before
				capturedLocal = after
			}
		}
		capturedArity := arity
		ef = func(ctx *Context) (Sequence, error) {
			ns := capturedNS
			if ns == "" {
				ns = nsFN
				if capturedPrefix != "" {
					var ok bool
					if ns, ok = ctx.Namespaces[capturedPrefix]; !ok {
						return nil, fmt.Errorf("could not find namespace for prefix %q", capturedPrefix)
					}
				}
			}
			fn := getfunction(ns, capturedLocal)
			if fn == nil {
				return nil, NewXPathError("XPST0017", fmt.Sprintf("unknown function %s#%d", capturedLocal, capturedArity))
			}
			return Sequence{&XPathFunction{
				Name:             capturedLocal,
				Namespace:        ns,
				Arity:            capturedArity,
				Fn:               fn.F,
				DynamicCallError: fn.DynamicCallError,
			}}, nil
		}
		leaveStep(tl, "41 parsePrimaryExpr (named-func-ref)")
		return ef, nil
	}

	// FunctionCall (QName or EQName followed by "(")
	if tl.nexttokIsTyp(tokOpenParen) {
		tl.unread() // function name
		if nexttok.Typ == tokQName {
			fname := nexttok.String()
			if fname == "text" || fname == "element" || fname == "attribute" || fname == "node" || fname == "comment" || fname == "processing-instruction" {
				return nil, nil
			}
		}
		ef, err := parseFunctionCall(tl)
		if err != nil {
			leaveStep(tl, "41 parsePrimaryExpr (err)")
			return nil, err
		}
		leaveStep(tl, "41 parsePrimaryExpr (fc)")
		return ef, nil
	}
	tl.unread()
	leaveStep(tl, "41 parsePrimaryExpr")
	return nil, nil
}

// [46] ParenthesizedExpr ::= "(" Expr? ")"
func parseParenthesizedExpr(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "46 parseParenthesizedExpr")
	var exp, ef EvalFunc
	var err error
	exp, err = parseExpr(tl)
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(tokCloseParen); err != nil {
		return nil, err
	}
	if exp == nil {
		// Empty parenthesized expression () = empty sequence
		leaveStep(tl, "46 parseParenthesizedExpr (empty)")
		return func(ctx *Context) (Sequence, error) {
			return Sequence{}, nil
		}, nil
	}
	ef = func(ctx *Context) (Sequence, error) {
		seq, err := exp(ctx)
		if err != nil {
			return nil, err
		}

		return seq, nil
	}

	leaveStep(tl, "46 parseParenthesizedExpr")
	return ef, nil
}

// [48] FunctionCall ::= QName "(" (ExprSingle ("," ExprSingle)*)? ")"
func parseFunctionCall(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "48 parseFunctionCall")
	var ef EvalFunc

	functionNameToken, err := tl.read()
	if err != nil {
		return nil, err
	}
	if err = tl.skipType(tokOpenParen); err != nil {
		return nil, err
	}
	fnName, ok := functionNameToken.Value.(string)
	if !ok {
		return nil, fmt.Errorf("expected function name, got %v", functionNameToken.Value)
	}
	fn := fnName
	// Pre-split function name to avoid splitting on every call.
	var fnPrefix, fnLocalName string
	var fnDirectNS string // set for EQName (Q{ns}local)
	if functionNameToken.Typ == tokEQName {
		if before, after, ok0 := strings.Cut(fn, "}"); ok0 {
			fnDirectNS = before
			fnLocalName = after
		}
	} else if before, after, ok0 := strings.Cut(fn, ":"); ok0 {
		fnPrefix = before
		fnLocalName = after
	} else {
		fnLocalName = fn
	}

	// callFn resolves the function by direct namespace or prefix
	callFn := func(ctx *Context, arguments []Sequence) (Sequence, error) {
		if fnDirectNS != "" {
			fnObj := getfunction(fnDirectNS, fnLocalName)
			if fnObj == nil {
				return nil, fmt.Errorf("Could not find function %q in namespace %q", fnLocalName, fnDirectNS)
			}
			return fnObj.F(ctx, arguments)
		}
		return callFunctionResolved(fnPrefix, fnLocalName, arguments, ctx)
	}

	if tl.nexttokIsTyp(tokCloseParen) {
		tl.read()
		ef = func(ctx *Context) (Sequence, error) {
			return callFn(ctx, []Sequence{})
		}
		leaveStep(tl, "48 parseFunctionCall (a)")
		return ef, nil
	}

	var efs []EvalFunc

	for {
		es, err := parseExprSingle(tl)
		if err != nil {
			return nil, err
		}
		efs = append(efs, es)
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read()
	}

	if err = tl.skipType(tokCloseParen); err != nil {
		leaveStep(tl, "48 parseFunctionCall (err)")
		return nil, fmt.Errorf("close paren expected")
	}

	// get expr single *
	ef = func(ctx *Context) (Sequence, error) {
		var arguments []Sequence
		saveContext := ctx.GetContextSequence()
		for i, es := range efs {
			if es == nil {
				return nil, fmt.Errorf("internal error: nil EvalFunc for argument %d of function %s:%s", i, fnPrefix, fnLocalName)
			}
			seq, err := es(ctx)
			if err != nil {
				return nil, err
			}
			arguments = append(arguments, seq)
			ctx.SetContextSequence(saveContext)
		}

		return callFn(ctx, arguments)
	}

	leaveStep(tl, "48 parseFunctionCall")
	return ef, nil
}

// orig: [50] SequenceType ::= ("empty-sequence" "(" ")")| (ItemType OccurrenceIndicator?)
// since empty-sequence() is implemented in 16 and 17, we can skip this here:
// [50] SequenceType ::= ItemType OccurrenceIndicator?
func parseSequenceType(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "50 parseSequenceType")
	var tf testFunc
	var err error

	tf, err = parseItemType(tl)
	if err != nil {
		leaveStep(tl, "50 parseSequenceType (err)")
		return nil, err
	}

	leaveStep(tl, "50 parseSequenceType")
	return tf, nil
}

// [52] ItemType ::= KindTest | ("item" "(" ")") | AtomicType
func parseItemType(tl *Tokenlist) (testFunc, error) {
	enterStep(tl, "52 parseItemType")
	var tf testFunc
	var err error

	if str, found := tl.readNexttokIfIsOneOfValueAndType(kindTestStrings, tokQName); found {
		if tf, err = parseKindTest(tl, str); err != nil {
			return nil, err
		}
		if tf != nil {
			return tf, nil
		}
	}

	// item() — matches any item
	if tl.readIfTokenFollow([]token{{"item", tokQName}, {'(', tokOpenParen}, {')', tokCloseParen}}) {
		tf = func(ctx *Context, itm Item) bool {
			return true
		}
		leaveStep(tl, "52 parseItemType (item)")
		return tf, nil
	}

	// map(*) / array(*)
	if tl.readIfTokenFollow([]token{{"map", tokQName}, {'(', tokOpenParen}, {"*", tokOperator}, {')', tokCloseParen}}) {
		tf = func(ctx *Context, itm Item) bool {
			_, ok := itm.(*XPathMap)
			return ok
		}
		leaveStep(tl, "52 parseItemType (map)")
		return tf, nil
	}
	if tl.readIfTokenFollow([]token{{"array", tokQName}, {'(', tokOpenParen}, {"*", tokOperator}, {')', tokCloseParen}}) {
		tf = func(ctx *Context, itm Item) bool {
			_, ok := itm.(*XPathArray)
			return ok
		}
		leaveStep(tl, "52 parseItemType (array)")
		return tf, nil
	}

	// function(*) — matches any function
	if tl.readIfTokenFollow([]token{{"function", tokQName}, {'(', tokOpenParen}, {"*", tokOperator}, {')', tokCloseParen}}) {
		tf = func(ctx *Context, itm Item) bool {
			_, ok := itm.(*XPathFunction)
			return ok
		}
		leaveStep(tl, "52 parseItemType (function)")
		return tf, nil
	}

	// AtomicType: xs:integer, xs:string, xs:double, xs:float, xs:decimal, xs:boolean, etc.
	nexttok, err := tl.peek()
	if err == nil && nexttok.Typ == tokQName {
		name, ok := nexttok.Value.(string)
		if ok {
			atomicType := resolveAtomicType(name)
			if atomicType != "" {
				tl.read() // consume the type name
				tf = makeAtomicTypeTest(atomicType)
				leaveStep(tl, "52 parseItemType (atomic)")
				return tf, nil
			}
		}
	}

	leaveStep(tl, "52 parseItemType")
	return tf, nil
}

// resolveAtomicType maps XPath type names to canonical type identifiers.
func resolveAtomicType(name string) string {
	// Handle prefixed and unprefixed forms
	switch name {
	case "xs:integer":
		return "integer"
	case "xs:int":
		return "int"
	case "xs:long":
		return "long"
	case "xs:short":
		return "short"
	case "xs:byte":
		return "byte"
	case "xs:unsignedLong":
		return "unsignedLong"
	case "xs:unsignedInt":
		return "unsignedInt"
	case "xs:unsignedShort":
		return "unsignedShort"
	case "xs:unsignedByte":
		return "unsignedByte"
	case "xs:nonPositiveInteger":
		return "nonPositiveInteger"
	case "xs:nonNegativeInteger":
		return "nonNegativeInteger"
	case "xs:negativeInteger":
		return "negativeInteger"
	case "xs:positiveInteger":
		return "positiveInteger"
	case "xs:double":
		return "double"
	case "xs:float":
		return "float"
	case "xs:decimal":
		return "decimal"
	case "xs:string":
		return "string"
	case "xs:normalizedString":
		return "normalizedString"
	case "xs:token":
		return "token"
	case "xs:language":
		return "language"
	case "xs:NMTOKEN":
		return "NMTOKEN"
	case "xs:Name":
		return "Name"
	case "xs:NCName":
		return "NCName"
	case "xs:ID":
		return "ID"
	case "xs:IDREF":
		return "IDREF"
	case "xs:ENTITY":
		return "ENTITY"
	case "xs:anyURI":
		return "anyURI"
	case "xs:untypedAtomic":
		return "untypedAtomic"
	case "xs:hexBinary":
		return "hexBinary"
	case "xs:base64Binary":
		return "base64Binary"
	case "xs:boolean":
		return "boolean"
	case "xs:dateTime":
		return "dateTime"
	case "xs:date":
		return "date"
	case "xs:time":
		return "time"
	case "xs:duration", "xs:dayTimeDuration", "xs:yearMonthDuration":
		return "duration"
	case "xs:gYear":
		return "gYear"
	case "xs:gMonth":
		return "gMonth"
	case "xs:gDay":
		return "gDay"
	case "xs:gYearMonth":
		return "gYearMonth"
	case "xs:gMonthDay":
		return "gMonthDay"
	case "xs:QName":
		return "qname"
	case "xs:numeric":
		return "numeric"
	}
	return ""
}

// makeAtomicTypeTest creates a testFunc for an atomic type check.
func makeAtomicTypeTest(atomicType string) testFunc {
	// Map atomicType name to IntSubtype for integer hierarchy checks
	intSubtypeByName := map[string]IntSubtype{
		"integer": IntInteger, "long": IntLong, "int": IntInt,
		"short": IntShort, "byte": IntByte,
		"nonNegativeInteger": IntNonNegativeInteger, "unsignedLong": IntUnsignedLong,
		"unsignedInt": IntUnsignedInt, "unsignedShort": IntUnsignedShort,
		"unsignedByte": IntUnsignedByte, "positiveInteger": IntPositiveInteger,
		"nonPositiveInteger": IntNonPositiveInteger, "negativeInteger": IntNegativeInteger,
	}
	strSubtypeByName := map[string]StrSubtype{
		"string": StrString, "normalizedString": StrNormalizedString,
		"token": StrToken, "language": StrLanguage, "NMTOKEN": StrNMTOKEN,
		"Name": StrName, "NCName": StrNCName, "ID": StrID,
		"IDREF": StrIDREF, "ENTITY": StrENTITY,
	}

	return func(ctx *Context, itm Item) bool {
		// Integer subtype hierarchy check
		if targetInt, ok := intSubtypeByName[atomicType]; ok {
			if v, ok := itm.(XSInteger); ok {
				return IntIsSubtypeOf(v.Subtype, targetInt)
			}
			// Bare int (from tokenizer) is xs:integer
			if _, ok := itm.(int); ok {
				return targetInt == IntInteger
			}
			return false
		}

		// String subtype hierarchy check
		if targetStr, ok := strSubtypeByName[atomicType]; ok {
			if v, ok := itm.(XSString); ok {
				return StrIsSubtypeOf(v.Subtype, targetStr)
			}
			// Bare string (from tokenizer/literals) is xs:string
			if _, ok := itm.(string); ok {
				return targetStr == StrString
			}
			return false
		}

		switch atomicType {
		case "double":
			switch itm.(type) {
			case XSDouble, float64:
				return true
			}
			return false
		case "float":
			_, ok := itm.(XSFloat)
			return ok
		case "decimal":
			// xs:decimal: XSDecimal, int, or XSInteger (integer is subtype of decimal)
			switch itm.(type) {
			case XSDecimal, int, XSInteger:
				return true
			}
			return false
		case "anyURI":
			_, ok := itm.(XSAnyURI)
			return ok
		case "untypedAtomic":
			_, ok := itm.(XSUntypedAtomic)
			return ok
		case "hexBinary":
			_, ok := itm.(XSHexBinary)
			return ok
		case "base64Binary":
			_, ok := itm.(XSBase64Binary)
			return ok
		case "boolean":
			_, ok := itm.(bool)
			return ok
		case "numeric":
			_, ok := ToFloat64(itm)
			return ok
		case "qname":
			_, ok := itm.(XSQName)
			return ok
		case "dateTime":
			_, ok := itm.(XSDateTime)
			return ok
		case "date":
			_, ok := itm.(XSDate)
			return ok
		case "time":
			_, ok := itm.(XSTime)
			return ok
		case "duration":
			_, ok := itm.(XSDuration)
			return ok
		case "gYear":
			_, ok := itm.(XSGYear)
			return ok
		case "gMonth":
			_, ok := itm.(XSGMonth)
			return ok
		case "gDay":
			_, ok := itm.(XSGDay)
			return ok
		case "gYearMonth":
			_, ok := itm.(XSGYearMonth)
			return ok
		case "gMonthDay":
			_, ok := itm.(XSGMonthDay)
			return ok
		}
		return false
	}
}

// [51] OccurrenceIndicator ::= "?" | "*" | "+"
// [53] AtomicType ::= QName
// [54] KindTest ::= DocumentTest|
// 					 ElementTest |
// 					 AttributeTest |
// 					 SchemaElementTest|
//			 		 SchemaAttributeTest|
//			 		 PITest|
//			 		 CommentTest|
//			 		 TextTest|
//			 		 AnyKindTest
// [67] ElementDeclaration ::= ElementName
// [56] DocumentTest ::= "document-node" "(" (ElementTest | SchemaElementTest)? ")"
// [60] AttributeTest ::= "attribute" "(" (AttribNameOrWildcard ("," TypeName)?)? ")"
// [61] AttribNameOrWildcard ::= AttributeName | "*"
// [68] AttributeName ::= QName
// [66] SchemaElementTest ::= "schema-element" "(" ElementDeclaration ")"
// [62] SchemaAttributeTest ::= "schema-attribute" "(" AttributeDeclaration ")"
// [63] AttributeDeclaration ::= AttributeName
// [59] PITest ::= "processing-instruction" "(" (NCName | StringLiteral)? ")"
// [64] ElementTest ::= "element" "(" (ElementNameOrWildcard ("," TypeName "?"?)?)? ")"
// [65] ElementNameOrWildcard ::= ElementName | "*"
// [69] ElementName ::= QName
// [70] TypeName ::= QName
// [58] CommentTest ::= "comment" "(" ")"
// [57] TextTest ::= "text" "(" ")"
// [55] AnyKindTest ::= "node" "(" ")"

var kindTestStrings = []string{"element", "node", "text", "attribute", "document-node", "schema-element", "schema-attribute", "processing-instruction", "comment"}

func parseKindTest(tl *Tokenlist, name string) (testFunc, error) {
	enterStep(tl, "54 parseKindTest")
	var tf testFunc
	var err error
	if err = tl.skipType(tokOpenParen); err != nil {
		return nil, err
	}
	switch name {
	case "element":
		nexttok, err := tl.peek()
		if err != nil {
			return nil, err
		}
		if nexttok.Value == ')' {
			tl.read()
			leaveStep(tl, "35 parseNodeTest")
			return isElement, nil
		} else if nexttok.Value == "*" && nexttok.Typ == tokOperator {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return isElement, nil
		} else if nexttok.Typ == tokEQName {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return returnElementEQNameTest(nexttok.String()), nil
		} else if nexttok.Typ == tokQName {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return returnElementNameTest(nexttok.String()), nil
		}
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		leaveStep(tl, "35 parseNodeTest")
		return isElement, nil
	case "node":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		tf := func(ctx *Context, itm Item) bool {
			return true
		}
		leaveStep(tl, "35 parseNodeTest")
		return tf, nil
	case "text":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		tf := func(ctx *Context, itm Item) bool {
			if _, ok := itm.(goxml.CharData); ok {
				return true
			}
			return false
		}
		leaveStep(tl, "35 parseNodeTest")
		return tf, nil
	case "attribute":
		nexttok, err := tl.peek()
		if err != nil {
			return nil, err
		}
		if nexttok.Value == ')' {
			tl.read()
			leaveStep(tl, "35 parseNodeTest")
			return isAttribute, nil
		} else if nexttok.Value == "*" && nexttok.Typ == tokOperator {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return isAttribute, nil
		} else if nexttok.Typ == tokQName {
			tl.read()
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return returnAttributeNameTest(nexttok.String()), nil
		}
	case "comment":
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}

		leaveStep(tl, "35 parseNodeTest")
		return isComment, nil
	case "processing-instruction":
		nexttok, err := tl.peek()
		if err != nil {
			return nil, err
		}
		if nexttok.Value == ')' {
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}

			leaveStep(tl, "35 parseNodeTest")
			return isProcessingInstruction, nil
		}
		nexttok, err = tl.read()
		if err != nil {
			return nil, err
		}
		if nexttok.Typ == tokQName {
			if err = tl.skipType(tokCloseParen); err != nil {
				return nil, err
			}
			leaveStep(tl, "35 parseNodeTest")
			return returnProcessingInstructionNameTest(nexttok.String()), nil
		}
	default:
		if err = tl.skipType(tokCloseParen); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("not implemented yet %s", name)
	}

	leaveStep(tl, "54 parseKindTest")
	return tf, nil
}

// parseMapConstructor parses key:value pairs after the opening { of map { ... }.
// The opening { has already been consumed.
func parseMapConstructor(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "parseMapConstructor")
	type kvPair struct {
		key   EvalFunc
		value EvalFunc
	}
	var pairs []kvPair

	if tl.nexttokIsTyp(tokCloseBrace) {
		tl.read() // consume }
		ef := func(ctx *Context) (Sequence, error) {
			return Sequence{&XPathMap{}}, nil
		}
		leaveStep(tl, "parseMapConstructor (empty)")
		return ef, nil
	}

	for {
		keyEf, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "parseMapConstructor (err key)")
			return nil, err
		}

		// expect colon separator
		if err = tl.skipType(tokOperator); err != nil {
			leaveStep(tl, "parseMapConstructor (err colon)")
			return nil, fmt.Errorf("':' expected in map constructor, got %v", err)
		}

		valueEf, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "parseMapConstructor (err value)")
			return nil, err
		}

		pairs = append(pairs, kvPair{key: keyEf, value: valueEf})

		if tl.nexttokIsTyp(tokComma) {
			tl.read() // consume comma
		} else {
			break
		}
	}

	if err := tl.skipType(tokCloseBrace); err != nil {
		leaveStep(tl, "parseMapConstructor (err close)")
		return nil, fmt.Errorf("'}' expected in map constructor")
	}

	ef := func(ctx *Context) (Sequence, error) {
		m := &XPathMap{}
		for _, pair := range pairs {
			keySeq, err := pair.key(ctx)
			if err != nil {
				return nil, err
			}
			if len(keySeq) != 1 {
				return nil, fmt.Errorf("map key must be a single item")
			}
			valueSeq, err := pair.value(ctx)
			if err != nil {
				return nil, err
			}
			m.Entries = append(m.Entries, MapEntry{Key: keySeq[0], Value: valueSeq})
		}
		return Sequence{m}, nil
	}
	leaveStep(tl, "parseMapConstructor")
	return ef, nil
}

// parseArrayConstructor parses a curly array constructor: array { Expr? }.
// The opening { has already been consumed.
// Per XPath 3.1: each item in the evaluated sequence becomes a separate member.
// parseSquareArrayConstructor parses: "[" (ExprSingle ("," ExprSingle)*)? "]"
// Each expression becomes one member of the array (its value is the full sequence).
func parseSquareArrayConstructor(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "parseSquareArrayConstructor")

	// Empty array: []
	if tl.nexttokIsTyp(tokCloseBracket) {
		tl.read() // consume ]
		ef := func(ctx *Context) (Sequence, error) {
			return Sequence{&XPathArray{}}, nil
		}
		leaveStep(tl, "parseSquareArrayConstructor (empty)")
		return ef, nil
	}

	var memberEfs []EvalFunc
	for {
		mef, err := parseExprSingle(tl)
		if err != nil {
			leaveStep(tl, "parseSquareArrayConstructor (err)")
			return nil, err
		}
		memberEfs = append(memberEfs, mef)
		if !tl.nexttokIsTyp(tokComma) {
			break
		}
		tl.read() // consume comma
	}

	if err := tl.skipType(tokCloseBracket); err != nil {
		leaveStep(tl, "parseSquareArrayConstructor (err close)")
		return nil, fmt.Errorf("']' expected in square array constructor")
	}

	ef := func(ctx *Context) (Sequence, error) {
		arr := &XPathArray{Members: make([]Sequence, len(memberEfs))}
		for i, mef := range memberEfs {
			seq, err := mef(ctx)
			if err != nil {
				return nil, err
			}
			arr.Members[i] = seq
		}
		return Sequence{arr}, nil
	}
	leaveStep(tl, "parseSquareArrayConstructor")
	return ef, nil
}

func parseArrayConstructor(tl *Tokenlist) (EvalFunc, error) {
	enterStep(tl, "parseArrayConstructor")

	if tl.nexttokIsTyp(tokCloseBrace) {
		tl.read() // consume }
		ef := func(ctx *Context) (Sequence, error) {
			return Sequence{&XPathArray{}}, nil
		}
		leaveStep(tl, "parseArrayConstructor (empty)")
		return ef, nil
	}

	contentEf, err := parseExpr(tl)
	if err != nil {
		leaveStep(tl, "parseArrayConstructor (err)")
		return nil, err
	}

	if err := tl.skipType(tokCloseBrace); err != nil {
		leaveStep(tl, "parseArrayConstructor (err close)")
		return nil, fmt.Errorf("'}' expected in array constructor")
	}

	ef := func(ctx *Context) (Sequence, error) {
		seq, err := contentEf(ctx)
		if err != nil {
			return nil, err
		}
		arr := &XPathArray{Members: make([]Sequence, len(seq))}
		for i, item := range seq {
			arr.Members[i] = Sequence{item}
		}
		return Sequence{arr}, nil
	}
	leaveStep(tl, "parseArrayConstructor")
	return ef, nil
}

// ParseXPath takes a previously created token list and returns a function that
// can be used to evaluate the XPath expression in different contexts.
func ParseXPath(tl *Tokenlist) (EvalFunc, error) {
	ef, err := parseExpr(tl)
	if err != nil {
		return nil, err
	}
	return ef, nil
}

// Parser contains all necessary references to the parser
type Parser struct {
	Ctx *Context
}

// XMLDocument returns the underlying XML document
func (xp *Parser) XMLDocument() *goxml.XMLDocument {
	return xp.Ctx.xmldoc
}

// SetVariable is used to set a variable name.
func (xp *Parser) SetVariable(name string, value Sequence) {
	xp.Ctx.vars[name] = value
}

// Evaluate reads an XPath expression and evaluates it in the given context.
// Parsed expressions are cached so that repeated evaluation of the same XPath
// string avoids re-tokenizing and re-parsing.
func (xp *Parser) Evaluate(xpath string) (Sequence, error) {
	// Reset per-evaluation state (XPath spec: current-dateTime is stable within one evaluation)
	xp.Ctx.currentTime = nil

	if cached, ok := exprCache.Load(xpath); ok {
		return cached.(EvalFunc)(xp.Ctx)
	}
	tl, err := stringToTokenlist(xpath)
	if err != nil {
		return nil, err
	}
	evaler, err := ParseXPath(tl)
	if err != nil {
		return nil, err
	}
	exprCache.Store(xpath, evaler)
	return evaler(xp.Ctx)
}

// NewParser returns a context to be filled
func NewParser(r io.Reader) (*Parser, error) {
	xp := &Parser{}

	doc, err := goxml.Parse(r)
	if err != nil {
		return nil, err
	}

	xp.Ctx = NewContext(doc)
	return xp, nil
}
