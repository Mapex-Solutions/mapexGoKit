package jsrunner

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

// DefaultTimeout is the maximum execution time for a script.
const DefaultTimeout = 10 * time.Second

// LabelValue represents a single option in a dropdown list.
type LabelValue struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// RunTransform executes an inline JS ES5 transform script.
//
// The script receives the HTTP response as `data` and must return an array
// of {label, value} objects.
//
// Example script:
//
//	function transform(data) {
//	  return data.result.map(function(item) {
//	    return { label: item.title, value: item.id };
//	  });
//	}
//
// Returns the transformed array of LabelValue items.
func RunTransform(ctx context.Context, script string, data interface{}) ([]LabelValue, error) {
	vm := goja.New()

	// Set the input data
	if err := vm.Set("data", data); err != nil {
		return nil, fmt.Errorf("jsrunner: failed to set data: %w", err)
	}

	// Timeout protection via context
	timeout := DefaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	// Execute: wrap script + call transform(data)
	program := script + "\n;transform(data);"

	val, err := vm.RunString(program)
	if err != nil {
		return nil, fmt.Errorf("jsrunner: script execution failed: %w", err)
	}

	// Extract result
	return extractLabelValues(vm, val)
}

// extractLabelValues converts a goja Value (expected to be an array of {label, value})
// into a Go slice of LabelValue.
func extractLabelValues(vm *goja.Runtime, val goja.Value) ([]LabelValue, error) {
	exported := val.Export()

	arr, ok := exported.([]interface{})
	if !ok {
		return nil, fmt.Errorf("jsrunner: transform must return an array, got %T", exported)
	}

	results := make([]LabelValue, 0, len(arr))
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("jsrunner: item[%d] must be an object with {label, value}, got %T", i, item)
		}

		label, _ := obj["label"]
		value, ok := obj["value"]
		if !ok {
			return nil, fmt.Errorf("jsrunner: item[%d] missing 'value' field", i)
		}

		labelStr := fmt.Sprintf("%v", label)
		results = append(results, LabelValue{
			Label: labelStr,
			Value: value,
		})
	}

	return results, nil
}
