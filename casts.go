package goxpath

import "fmt"

var (
	// ErrConversion is returned in case of an usuccessful cast.
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
