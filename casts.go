package goxpath

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrConversion is returned in case of an unsuccessful cast.
	ErrConversion = fmt.Errorf("conversion failed")
)

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
	var sb strings.Builder
	if d.Negative {
		sb.WriteByte('-')
	}
	sb.WriteByte('P')
	if d.Years != 0 {
		fmt.Fprintf(&sb, "%dY", d.Years)
	}
	if d.Months != 0 {
		fmt.Fprintf(&sb, "%dM", d.Months)
	}
	if d.Days != 0 {
		fmt.Fprintf(&sb, "%dD", d.Days)
	}
	if d.Hours != 0 || d.Minutes != 0 || d.Seconds != 0 {
		sb.WriteByte('T')
		if d.Hours != 0 {
			fmt.Fprintf(&sb, "%dH", d.Hours)
		}
		if d.Minutes != 0 {
			fmt.Fprintf(&sb, "%dM", d.Minutes)
		}
		if d.Seconds != 0 {
			s := strconv.FormatFloat(d.Seconds, 'f', -1, 64)
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
		return d, fmt.Errorf("FORG0001: empty duration string")
	}
	if s[0] == '-' {
		d.Negative = true
		s = s[1:]
	}
	if len(s) == 0 || s[0] != 'P' {
		return d, fmt.Errorf("FORG0001: duration must start with 'P': %q", orig)
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
			return d, fmt.Errorf("FORG0001: invalid duration format: %q", orig)
		}
		numStr := s[:numEnd]
		designator := s[numEnd]
		s = s[numEnd+1:]

		if designator == 'S' || strings.Contains(numStr, ".") {
			f, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return d, fmt.Errorf("FORG0001: invalid number in duration: %q", orig)
			}
			if designator == 'S' {
				d.Seconds = f
			} else {
				return d, fmt.Errorf("FORG0001: decimal only allowed for seconds: %q", orig)
			}
		} else {
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return d, fmt.Errorf("FORG0001: invalid number in duration: %q", orig)
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
				return d, fmt.Errorf("FORG0001: unexpected designator '%c' in duration: %q", designator, orig)
			}
		}
	}
	return d, nil
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
	default:
		panic(fmt.Sprintf("nyi xs:integer %T", itm))
	}
}

func xsTime(ctx *Context, args []Sequence) (Sequence, error) {
	firstarg, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}

	t, err := time.Parse("15:04:05-07:00", firstarg)
	if err != nil {
		t, err = time.Parse("15:04:05", firstarg)
		if err != nil {
			return nil, err
		}
	}
	return Sequence{XSTime(t)}, nil
}

func xsDouble(ctx *Context, args []Sequence) (Sequence, error) {
	nv, err := NumberValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{nv}, nil
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

	return nil, fmt.Errorf("FORG0001: cannot cast %q to xs:date", firstarg)
}

func xsInteger(ctx *Context, args []Sequence) (Sequence, error) {
	nv, err := NumberValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{int(nv)}, nil
}

func xsString(ctx *Context, args []Sequence) (Sequence, error) {
	sv, err := StringValue(args[0])
	if err != nil {
		return nil, err
	}
	return Sequence{sv}, nil
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
	return nil, fmt.Errorf("FORG0001: cannot cast %q to xs:dateTime", firstarg)
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

func init() {
	RegisterFunction(&Function{Name: "time", Namespace: nsXS, F: xsTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "dateTime", Namespace: nsXS, F: xsDateTime, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "double", Namespace: nsXS, F: xsDouble, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "duration", Namespace: nsXS, F: xsDuration, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "date", Namespace: nsXS, F: xsDate, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "integer", Namespace: nsXS, F: xsInteger, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "string", Namespace: nsXS, F: xsString, MinArg: 1, MaxArg: 1})
}
