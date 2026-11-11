package golden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalPlan preserves all plan fields except its generation timestamp and
// orders only Terraform JSON arrays that represent unordered resource sets.
func CanonicalPlan(data []byte) ([]byte, error) {
	value, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("plan JSON root must be an object")
	}
	root["timestamp"] = nil
	canonicalizePlan(value)
	return marshalJSON(value)
}

// CanonicalRelay normalizes only generated resource IDs and ordering of relay
// resource collections. Attribute values and sensitive paths remain intact.
func CanonicalRelay(data []byte) ([]byte, error) {
	value, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	canonicalizeRelay(value)
	return marshalJSON(value)
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func marshalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	return append(data, '\n'), nil
}

func canonicalizePlan(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			canonicalizePlan(child)
			switch key {
			case "resource_changes", "resource_drift":
				sortObjects(child, "address")
			case "resources", "child_modules":
				sortObjects(child, "address")
			}
		}
	case []any:
		for _, child := range value {
			canonicalizePlan(child)
		}
	}
}

func canonicalizeRelay(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "__id__" {
				value[key] = "<generated>"
				continue
			}
			canonicalizeRelay(child)
			if key == "resources" {
				sortObjects(child, "type", "name")
			}
			if key == "instances" {
				sortObjects(child, "index_key")
			}
		}
	case []any:
		for _, child := range value {
			canonicalizeRelay(child)
		}
	}
}

func sortObjects(value any, keys ...string) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, leftOK := items[i].(map[string]any)
		right, rightOK := items[j].(map[string]any)
		if !leftOK || !rightOK {
			return false
		}
		for _, key := range keys {
			leftValue, rightValue := fmt.Sprint(left[key]), fmt.Sprint(right[key])
			if leftValue != rightValue {
				return leftValue < rightValue
			}
		}
		return false
	})
}
