package showjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/kamichidu/go-tfrelay"
)

type jsonValue struct {
	raw  json.RawMessage
	obj  map[string]jsonValue
	arr  []jsonValue
	kind string
	null bool
}

func parseValue(raw json.RawMessage) (jsonValue, error) {
	v := jsonValue{raw: append(json.RawMessage(nil), raw...)}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return v, errors.New("empty JSON value")
	}
	switch trimmed[0] {
	case '{':
		v.kind = "object"
		var rawObject map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &rawObject); err != nil {
			return v, err
		}
		v.obj = make(map[string]jsonValue, len(rawObject))
		for key, rawChild := range rawObject {
			child, err := parseValue(rawChild)
			if err != nil {
				return v, err
			}
			v.obj[key] = child
		}
	case '[':
		v.kind = "array"
		var rawArray []json.RawMessage
		if err := json.Unmarshal(trimmed, &rawArray); err != nil {
			return v, err
		}
		v.arr = make([]jsonValue, len(rawArray))
		for i, rawChild := range rawArray {
			child, err := parseValue(rawChild)
			if err != nil {
				return v, err
			}
			v.arr[i] = child
		}
	case 'n':
		v.kind, v.null = "null", true
	case 't', 'f':
		v.kind = "boolean"
	case '"':
		v.kind = "string"
	default:
		v.kind = "number"
	}
	return v, nil
}

type snapshotEnvelope struct {
	FormatVersion   string          `json:"format_version"`
	Errored         *bool           `json:"errored"`
	Complete        *bool           `json:"complete"`
	DeferredChanges json.RawMessage `json:"deferred_changes"`
	PlannedValues   struct {
		RootModule json.RawMessage `json:"root_module"`
	} `json:"planned_values"`
	Values struct {
		RootModule json.RawMessage `json:"root_module"`
	} `json:"values"`
	ResourceChanges []struct {
		Address      string `json:"address"`
		ProviderName string `json:"provider_name"`
		Change       struct {
			AfterUnknown   json.RawMessage `json:"after_unknown"`
			AfterSensitive json.RawMessage `json:"after_sensitive"`
		} `json:"change"`
	} `json:"resource_changes"`
}

type moduleValue struct {
	Resources    []json.RawMessage `json:"resources"`
	ChildModules []json.RawMessage `json:"child_modules"`
}

type resourceValue struct {
	Address         string          `json:"address"`
	Type            string          `json:"type"`
	Name            string          `json:"name"`
	ProviderName    string          `json:"provider_name"`
	Index           json.RawMessage `json:"index"`
	Values          json.RawMessage `json:"values"`
	SensitiveValues json.RawMessage `json:"sensitive_values"`
}

type instanceMasks struct{ unknown, sensitive json.RawMessage }

func DecodeSnapshot(input []byte, planned bool) (tfrelay.RelayData, error) {
	var out tfrelay.RelayData
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var envelope snapshotEnvelope
	if err := dec.Decode(&envelope); err != nil {
		return out, err
	}
	if envelope.FormatVersion != "" {
		major := strings.SplitN(envelope.FormatVersion, ".", 2)[0]
		if major != "1" {
			return out, fmt.Errorf("unsupported Terraform show JSON format_version %q", envelope.FormatVersion)
		}
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return out, errors.New("trailing JSON data")
	}
	root := envelope.Values.RootModule
	if planned {
		if envelope.Errored != nil && *envelope.Errored {
			return out, errors.New("Terraform plan has errored=true")
		}
		if envelope.Complete != nil && !*envelope.Complete {
			return out, errors.New("Terraform plan has complete=false")
		}
		if len(envelope.DeferredChanges) > 0 && string(envelope.DeferredChanges) != "null" {
			var deferred []json.RawMessage
			if err := json.Unmarshal(envelope.DeferredChanges, &deferred); err != nil {
				return out, errors.New("Terraform plan deferred_changes must be an array")
			}
			if len(deferred) != 0 {
				return out, errors.New("Terraform plan has nonempty deferred_changes")
			}
		}
		root = envelope.PlannedValues.RootModule
	}
	if len(root) == 0 || string(root) == "null" {
		return out, errors.New("show JSON omitted root module")
	}
	out.IRVersion = 2
	if planned {
		out.SnapshotKind = "planned"
	} else {
		out.SnapshotKind = "state"
	}
	masks := map[string]instanceMasks{}
	if planned {
		for _, rc := range envelope.ResourceChanges {
			masks[rc.Address] = instanceMasks{unknown: rc.Change.AfterUnknown, sensitive: rc.Change.AfterSensitive}
		}
	}
	groups := map[string]*tfrelay.Resource{}
	providers := map[string]string{}
	for _, rc := range envelope.ResourceChanges {
		if rc.ProviderName != "" {
			providers[rc.Address] = rc.ProviderName
		}
	}
	if err := walkModule(root, masks, providers, groups); err != nil {
		return out, err
	}
	for _, resource := range groups {
		out.Resources = append(out.Resources, *resource)
	}
	return out, nil
}

func walkModule(raw json.RawMessage, masks map[string]instanceMasks, providers map[string]string, groups map[string]*tfrelay.Resource) error {
	var module moduleValue
	if err := json.Unmarshal(raw, &module); err != nil {
		return err
	}
	for _, rawResource := range module.Resources {
		var resource resourceValue
		if err := json.Unmarshal(rawResource, &resource); err != nil {
			return err
		}
		if resource.Address == "" {
			return errors.New("planned resource omitted address")
		}
		attrs, err := parseValue(resource.Values)
		if err != nil {
			return fmt.Errorf("resource %s values: %w", resource.Address, err)
		}
		resourceMask := masks[resource.Address]
		mask, err := parseOptionalValue(resourceMask.unknown)
		if err != nil {
			return err
		}
		sensitive, err := parseOptionalValue(resource.SensitiveValues)
		if err != nil {
			return err
		}
		valueMap := attrs.obj
		ok := valueMap != nil
		if !ok || attrs.kind != "object" {
			return fmt.Errorf("resource %s values must be an object", resource.Address)
		}
		changeSensitiveValue, err := parseOptionalValue(resourceMask.sensitive)
		if err != nil {
			return err
		}
		changeSensitive, err := mergeMaskObjects(sensitive, changeSensitiveValue)
		if err != nil {
			return err
		}
		merged := make(map[string]tfrelay.Value, len(valueMap))
		keys := map[string]bool{}
		for k := range valueMap {
			keys[k] = true
		}
		for k := range maskChildren(mask) {
			keys[k] = true
		}
		for k := range maskChildren(changeSensitive) {
			keys[k] = true
		}
		for key := range keys {
			v, present := valueMap[key]
			unknownMask := childMask(mask, key)
			sensitiveMask := childMask(changeSensitive, key)
			if !present && !maskHasTrue(unknownMask) {
				continue
			}
			merged[key], err = normalizeValue(v, present, unknownMask, sensitiveMask, false)
			if err != nil {
				return fmt.Errorf("resource %s attribute %s: %w", resource.Address, key, err)
			}
		}
		provider := resource.ProviderName
		if provider == "" {
			provider = providers[resource.Address]
		}
		groupKey := resourceBlockAddress(resource.Address) + "\x00" + provider
		group := groups[groupKey]
		if group == nil {
			group = &tfrelay.Resource{ResourceID: uuid.NewString(), Type: resource.Type, Name: resource.Name, Provider: provider}
			groups[groupKey] = group
		}
		instance := tfrelay.Instance{Address: resource.Address, Attributes: merged}
		if len(resource.Index) != 0 && string(resource.Index) != "null" {
			token, err := canonicalScalar(resource.Index)
			if err != nil {
				return err
			}
			instance.IndexKey = token
		}
		group.Instances = append(group.Instances, instance)
	}
	for _, child := range module.ChildModules {
		if err := walkModule(child, masks, providers, groups); err != nil {
			return err
		}
	}
	return nil
}

func resourceBlockAddress(address string) string {
	lastDot := strings.LastIndex(address, ".")
	if bracket := strings.LastIndex(address, "["); bracket > lastDot && strings.HasSuffix(address, "]") {
		return address[:bracket]
	}
	return address
}

func parseOptionalValue(raw json.RawMessage) (jsonValue, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return jsonValue{}, nil
	}
	return parseValue(raw)
}

func mergeMaskObjects(a, b jsonValue) (jsonValue, error) {
	if a.kind == "" {
		return b, nil
	}
	if b.kind == "" {
		return a, nil
	}
	if a.kind != "object" || b.kind != "object" {
		return jsonValue{}, errors.New("mask must be an object")
	}
	out := jsonValue{kind: "object", obj: map[string]jsonValue{}}
	for k, v := range a.obj {
		out.obj[k] = v
	}
	for k, v := range b.obj {
		if prior, ok := out.obj[k]; ok && prior.kind == "object" && v.kind == "object" {
			merged, err := mergeMaskObjects(prior, v)
			if err != nil {
				return out, err
			}
			out.obj[k] = merged
		} else if ok && maskTrue(prior) {
			continue
		} else {
			out.obj[k] = v
		}
	}
	return out, nil
}
func maskChildren(v jsonValue) map[string]jsonValue {
	if v.kind == "object" {
		return v.obj
	}
	return map[string]jsonValue{}
}
func childMask(v jsonValue, key string) jsonValue {
	if v.kind == "object" {
		return v.obj[key]
	}
	return jsonValue{}
}
func maskTrue(v jsonValue) bool {
	if v.kind != "boolean" {
		return false
	}
	var b bool
	_ = json.Unmarshal(v.raw, &b)
	return b
}
func maskHasTrue(v jsonValue) bool {
	if maskTrue(v) {
		return true
	}
	if v.kind == "object" {
		for _, c := range v.obj {
			if maskHasTrue(c) {
				return true
			}
		}
	}
	if v.kind == "array" {
		for _, c := range v.arr {
			if maskHasTrue(c) {
				return true
			}
		}
	}
	return false
}
func normalizeValue(v jsonValue, present bool, unknown, sensitive jsonValue, inheritedSensitive bool) (tfrelay.Value, error) {
	out := tfrelay.Value{Sensitive: maskTrue(sensitive) || inheritedSensitive}
	if maskTrue(unknown) {
		out.State = "unknown"
		return out, nil
	}
	if !present {
		return materializeUnknownMask(unknown, sensitive, inheritedSensitive)
	}
	out.State, out.Type = "known", v.kind
	if v.kind == "object" {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(v.raw, &object); err != nil {
			return out, err
		}
		values := make(map[string]tfrelay.Value, len(object))
		for key, raw := range object {
			child, err := parseValue(raw)
			if err != nil {
				return out, err
			}
			values[key], err = normalizeValue(child, true, childMask(unknown, key), childMask(sensitive, key), out.Sensitive)
			if err != nil {
				return out, err
			}
		}
		for key, unknownChild := range maskChildren(unknown) {
			if _, ok := values[key]; !ok && maskHasTrue(unknownChild) {
				values[key] = tfrelay.Value{State: "unknown", Sensitive: maskTrue(childMask(sensitive, key)) || out.Sensitive}
			}
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return out, err
		}
		out.Value = encoded
		return out, nil
	}
	if v.kind == "array" {
		values := make([]tfrelay.Value, len(v.arr))
		for i, child := range v.arr {
			var unknownChild, sensitiveChild jsonValue
			if i < len(unknown.arr) {
				unknownChild = unknown.arr[i]
			}
			if i < len(sensitive.arr) {
				sensitiveChild = sensitive.arr[i]
			}
			normalized, err := normalizeValue(child, true, unknownChild, sensitiveChild, out.Sensitive)
			if err != nil {
				return out, err
			}
			values[i] = normalized
		}
		for i, unknownChild := range unknown.arr {
			if i >= len(values) && maskHasTrue(unknownChild) {
				values = append(values, tfrelay.Value{State: "unknown", Sensitive: out.Sensitive})
			}
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return out, err
		}
		out.Value = encoded
		return out, nil
	}
	out.Value = append(json.RawMessage(nil), v.raw...)
	return out, nil
}

func materializeUnknownMask(unknown, sensitive jsonValue, inheritedSensitive bool) (tfrelay.Value, error) {
	if maskTrue(unknown) {
		return tfrelay.Value{State: "unknown", Sensitive: maskTrue(sensitive) || inheritedSensitive}, nil
	}
	switch unknown.kind {
	case "object":
		children := make(map[string]tfrelay.Value)
		for key, mask := range unknown.obj {
			if !maskHasTrue(mask) {
				continue
			}
			child, err := materializeUnknownMask(mask, childMask(sensitive, key), inheritedSensitive || maskTrue(sensitive))
			if err != nil {
				return tfrelay.Value{}, err
			}
			children[key] = child
		}
		if len(children) == 0 {
			return tfrelay.Value{}, nil
		}
		encoded, err := json.Marshal(children)
		if err != nil {
			return tfrelay.Value{}, err
		}
		return tfrelay.Value{State: "known", Type: "object", Sensitive: inheritedSensitive, Value: encoded}, nil
	case "array":
		children := make([]tfrelay.Value, len(unknown.arr))
		for i, mask := range unknown.arr {
			var sensitiveChild jsonValue
			if i < len(sensitive.arr) {
				sensitiveChild = sensitive.arr[i]
			}
			if maskHasTrue(mask) {
				child, err := materializeUnknownMask(mask, sensitiveChild, inheritedSensitive)
				if err != nil {
					return tfrelay.Value{}, err
				}
				children[i] = child
			} else {
				children[i] = tfrelay.Value{State: "unknown", Sensitive: inheritedSensitive}
			}
		}
		encoded, err := json.Marshal(children)
		if err != nil {
			return tfrelay.Value{}, err
		}
		return tfrelay.Value{State: "known", Type: "array", Sensitive: inheritedSensitive, Value: encoded}, nil
	}
	return tfrelay.Value{}, nil
}

func canonicalScalar(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || (trimmed[0] != '"' && trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return "", errors.New("instance index must be a string or number")
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", err
		}
		encoded, _ := json.Marshal(s)
		return string(encoded), nil
	}
	var number json.Number
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return "", errors.New("invalid numeric instance index")
	}
	return string(trimmed), nil
}
