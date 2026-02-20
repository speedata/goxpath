package goxpath

import "fmt"

const nsMap = "http://www.w3.org/2005/xpath-functions/map"

// MapEntry represents a key-value pair in an XPath map.
type MapEntry struct {
	Key   Item
	Value Sequence
}

// XPathMap represents an XPath 3.1 map.
type XPathMap struct {
	Entries []MapEntry
}

// Get looks up a key in the map by comparing string values.
func (m *XPathMap) Get(key Item) (Sequence, bool) {
	keyStr := itemStringvalue(key)
	for _, entry := range m.Entries {
		if itemStringvalue(entry.Key) == keyStr {
			return entry.Value, true
		}
	}
	return nil, false
}

// Keys returns all keys in the map as a Sequence.
func (m *XPathMap) Keys() Sequence {
	seq := make(Sequence, len(m.Entries))
	for i, entry := range m.Entries {
		seq[i] = entry.Key
	}
	return seq
}

// Size returns the number of entries in the map.
func (m *XPathMap) Size() int {
	return len(m.Entries)
}

// Contains checks if a key exists in the map.
func (m *XPathMap) Contains(key Item) bool {
	_, found := m.Get(key)
	return found
}

func fnMapGet(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("map:get expects a single map as first argument")
	}
	m, ok := args[0][0].(*XPathMap)
	if !ok {
		return nil, fmt.Errorf("map:get expects a map as first argument, got %T", args[0][0])
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("map:get expects a single key as second argument")
	}
	val, found := m.Get(args[1][0])
	if !found {
		return Sequence{}, nil
	}
	return val, nil
}

func fnMapKeys(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("map:keys expects a single map as first argument")
	}
	m, ok := args[0][0].(*XPathMap)
	if !ok {
		return nil, fmt.Errorf("map:keys expects a map as first argument, got %T", args[0][0])
	}
	return m.Keys(), nil
}

func fnMapContains(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("map:contains expects a single map as first argument")
	}
	m, ok := args[0][0].(*XPathMap)
	if !ok {
		return nil, fmt.Errorf("map:contains expects a map as first argument, got %T", args[0][0])
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("map:contains expects a single key as second argument")
	}
	return Sequence{m.Contains(args[1][0])}, nil
}

func fnMapSize(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("map:size expects a single map as first argument")
	}
	m, ok := args[0][0].(*XPathMap)
	if !ok {
		return nil, fmt.Errorf("map:size expects a map as first argument, got %T", args[0][0])
	}
	return Sequence{m.Size()}, nil
}

func fnMapPut(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) != 1 {
		return nil, fmt.Errorf("map:put expects a single map as first argument")
	}
	m, ok := args[0][0].(*XPathMap)
	if !ok {
		return nil, fmt.Errorf("map:put expects a map as first argument, got %T", args[0][0])
	}
	if len(args[1]) != 1 {
		return nil, fmt.Errorf("map:put expects a single key as second argument")
	}
	key := args[1][0]
	value := args[2]

	// Create a new map with the entry added/replaced
	keyStr := itemStringvalue(key)
	newEntries := make([]MapEntry, 0, len(m.Entries)+1)
	replaced := false
	for _, entry := range m.Entries {
		if itemStringvalue(entry.Key) == keyStr {
			newEntries = append(newEntries, MapEntry{Key: key, Value: value})
			replaced = true
		} else {
			newEntries = append(newEntries, entry)
		}
	}
	if !replaced {
		newEntries = append(newEntries, MapEntry{Key: key, Value: value})
	}
	return Sequence{&XPathMap{Entries: newEntries}}, nil
}

func fnMapMerge(ctx *Context, args []Sequence) (Sequence, error) {
	// args[0] is a sequence of maps
	result := &XPathMap{}
	seen := make(map[string]bool)

	for _, itm := range args[0] {
		m, ok := itm.(*XPathMap)
		if !ok {
			return nil, fmt.Errorf("map:merge expects a sequence of maps, got %T", itm)
		}
		for _, entry := range m.Entries {
			keyStr := itemStringvalue(entry.Key)
			if !seen[keyStr] {
				seen[keyStr] = true
				result.Entries = append(result.Entries, entry)
			}
		}
	}
	return Sequence{result}, nil
}

func init() {
	RegisterFunction(&Function{Name: "get", Namespace: nsMap, F: fnMapGet, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "keys", Namespace: nsMap, F: fnMapKeys, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "contains", Namespace: nsMap, F: fnMapContains, MinArg: 2, MaxArg: 2})
	RegisterFunction(&Function{Name: "size", Namespace: nsMap, F: fnMapSize, MinArg: 1, MaxArg: 1})
	RegisterFunction(&Function{Name: "put", Namespace: nsMap, F: fnMapPut, MinArg: 3, MaxArg: 3})
	RegisterFunction(&Function{Name: "merge", Namespace: nsMap, F: fnMapMerge, MinArg: 1, MaxArg: 1})
}
