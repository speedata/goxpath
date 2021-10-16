package xpath

var xpathfunctions map[string]*Function

const (
	fnNS = "http://www.w3.org/2005/xpath-functions"
)

func fnTrue(s sequence) sequence {
	return sequence{true}
}

func fnFalse(s sequence) sequence {
	return sequence{false}
}

func init() {
	xpathfunctions = make(map[string]*Function)

	RegisterFunction(&Function{Name: "true", Namespace: fnNS, F: fnTrue})
	RegisterFunction(&Function{Name: "false", Namespace: fnNS, F: fnFalse})

}

// Function represents an XPath function
type Function struct {
	Name      string
	Namespace string
	F         func(sequence) sequence
	MinArg    int
	MaxArg    int
}

// RegisterFunction registers an XPath function
func RegisterFunction(f *Function) {
	xpathfunctions[f.Name] = f
}

func getfunction(name string) *Function {
	return xpathfunctions[name]
}
