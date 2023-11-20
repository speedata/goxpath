package goxpath

import (
	"fmt"
	"time"
)

var (
	// ErrConversion is returned in case of an unsuccessful cast.
	ErrConversion = fmt.Errorf("conversion failed")
)

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

func init() {
	RegisterFunction(&Function{Name: "time", Namespace: nsXS, F: xsTime, MinArg: 1, MaxArg: 1})
}
