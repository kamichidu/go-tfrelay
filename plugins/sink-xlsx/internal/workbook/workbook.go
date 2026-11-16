package workbook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kamichidu/go-tfrelay/plugins/sink-xlsx/internal/projection"
	"github.com/xuri/excelize/v2"
)

const (
	redacted = "[REDACTED]"
	unknown  = "<unknown>"
)

type row struct{ resource, instance, path, value, state string }

// Write emits a stable, flattened workbook. Rows are sorted by resource address,
// instance address, and attribute path; maps therefore cannot affect output order.
func Write(data projection.RelayData, out *bytes.Buffer) error {
	if data.IRVersion != 1 || (data.SnapshotKind != "planned" && data.SnapshotKind != "state") {
		return fmt.Errorf("unsupported RelayData contract: ir_version=%d snapshot_kind=%q", data.IRVersion, data.SnapshotKind)
	}
	rows, err := rowsFor(data)
	if err != nil {
		return err
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	const sheet = "Resources"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"resource_address", "instance_address", "attribute_path", "value", "state"}
	for col, value := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := f.SetCellStr(sheet, cell, value); err != nil {
			return err
		}
	}
	for i, r := range rows {
		values := []string{r.resource, r.instance, r.path, r.value, r.state}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+2)
			if err := f.SetCellStr(sheet, cell, value); err != nil {
				return err
			}
		}
	}
	for _, column := range []struct {
		name  string
		width float64
	}{{"A", 36}, {"B", 48}, {"C", 48}, {"D", 64}, {"E", 14}} {
		if err := f.SetColWidth(sheet, column.name, column.name, column.width); err != nil {
			return err
		}
	}
	return f.Write(out)
}

func rowsFor(data projection.RelayData) ([]row, error) {
	resources := append([]projection.Resource(nil), data.Resources...)
	rows := make([]row, 0)
	for _, resource := range resources {
		if resource.Type == "" || resource.Name == "" {
			return nil, errors.New("resource omitted type or name")
		}
		instances := append([]projection.Instance(nil), resource.Instances...)
		sort.Slice(instances, func(i, j int) bool { return instances[i].Address < instances[j].Address })
		for _, instance := range instances {
			if instance.Address == "" {
				return nil, fmt.Errorf("resource %s omitted instance address", resource.Type+"."+resource.Name)
			}
			resourceAddress := instance.Address
			if bracket := strings.LastIndex(resourceAddress, "["); bracket >= 0 && strings.HasSuffix(resourceAddress, "]") {
				resourceAddress = resourceAddress[:bracket]
			}
			keys := make([]string, 0, len(instance.Attributes))
			for key := range instance.Attributes {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if err := appendValue(&rows, resourceAddress, instance.Address, key, instance.Attributes[key]); err != nil {
					return nil, fmt.Errorf("attribute %s.%s: %w", instance.Address, key, err)
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.resource != b.resource {
			return a.resource < b.resource
		}
		if a.instance != b.instance {
			return a.instance < b.instance
		}
		return a.path < b.path
	})
	return rows, nil
}

func appendValue(rows *[]row, resource, instance, path string, value projection.Value) error {
	if value.Sensitive {
		state := value.State
		if state == "" {
			state = "known"
		}
		*rows = append(*rows, row{resource, instance, path, redacted, state})
		return nil
	}
	switch value.State {
	case "unknown":
		*rows = append(*rows, row{resource, instance, path, unknown, "unknown"})
		return nil
	case "known":
	default:
		return fmt.Errorf("unsupported value state %q", value.State)
	}
	switch value.Type {
	case "object":
		var children map[string]projection.Value
		if err := json.Unmarshal(value.Value, &children); err != nil {
			return fmt.Errorf("decode object: %w", err)
		}
		keys := make([]string, 0, len(children))
		for key := range children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			*rows = append(*rows, row{resource, instance, path, "{}", "known"})
		}
		for _, key := range keys {
			if err := appendValue(rows, resource, instance, path+"."+key, children[key]); err != nil {
				return err
			}
		}
	case "array":
		var children []projection.Value
		if err := json.Unmarshal(value.Value, &children); err != nil {
			return fmt.Errorf("decode array: %w", err)
		}
		if len(children) == 0 {
			*rows = append(*rows, row{resource, instance, path, "[]", "known"})
		}
		for i, child := range children {
			if err := appendValue(rows, resource, instance, path+"["+strconv.Itoa(i)+"]", child); err != nil {
				return err
			}
		}
	default:
		var compact bytes.Buffer
		if len(bytes.TrimSpace(value.Value)) == 0 {
			return errors.New("known value has no JSON value")
		}
		if err := json.Compact(&compact, value.Value); err != nil {
			return fmt.Errorf("invalid JSON value: %w", err)
		}
		text := compact.String()
		if strings.HasPrefix(text, "\"") {
			if err := json.Unmarshal(compact.Bytes(), &text); err != nil {
				return err
			}
		}
		*rows = append(*rows, row{resource, instance, path, text, "known"})
	}
	return nil
}
