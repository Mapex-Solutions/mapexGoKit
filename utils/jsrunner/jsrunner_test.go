package jsrunner

import (
	"context"
	"testing"
	"time"
)

func TestRunTransform_SimpleMap(t *testing.T) {
	script := `
function transform(data) {
	return data.result.map(function(item) {
		return { label: item.title, value: item.id };
	});
}
`
	data := map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{"id": 1, "title": "Alpha"},
			map[string]interface{}{"id": 2, "title": "Beta"},
			map[string]interface{}{"id": 3, "title": "Gamma"},
		},
	}

	results, err := RunTransform(context.Background(), script, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Label != "Alpha" {
		t.Errorf("expected label 'Alpha', got %q", results[0].Label)
	}
	if results[0].Value != int64(1) {
		t.Errorf("expected value 1, got %v (%T)", results[0].Value, results[0].Value)
	}
}

func TestRunTransform_FilterAndConcat(t *testing.T) {
	script := `
function transform(data) {
	return data.users.filter(function(u) {
		return !u.is_bot;
	}).map(function(u) {
		return { label: u.first_name + " " + u.last_name, value: String(u.id) };
	});
}
`
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"id": 100, "first_name": "John", "last_name": "Doe", "is_bot": false},
			map[string]interface{}{"id": 200, "first_name": "Bot", "last_name": "Helper", "is_bot": true},
			map[string]interface{}{"id": 300, "first_name": "Jane", "last_name": "Smith", "is_bot": false},
		},
	}

	results, err := RunTransform(context.Background(), script, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (bots filtered), got %d", len(results))
	}
	if results[0].Label != "John Doe" {
		t.Errorf("expected 'John Doe', got %q", results[0].Label)
	}
	if results[1].Label != "Jane Smith" {
		t.Errorf("expected 'Jane Smith', got %q", results[1].Label)
	}
}

func TestRunTransform_FlattenNested(t *testing.T) {
	script := `
function transform(data) {
	var result = [];
	for (var i = 0; i < data.teams.length; i++) {
		var team = data.teams[i];
		for (var j = 0; j < team.members.length; j++) {
			var m = team.members[j];
			result.push({ label: team.name + " / " + m.name, value: m.id });
		}
	}
	return result;
}
`
	data := map[string]interface{}{
		"teams": []interface{}{
			map[string]interface{}{
				"name": "Engineering",
				"members": []interface{}{
					map[string]interface{}{"id": "a1", "name": "Alice"},
					map[string]interface{}{"id": "a2", "name": "Bob"},
				},
			},
			map[string]interface{}{
				"name": "Design",
				"members": []interface{}{
					map[string]interface{}{"id": "b1", "name": "Carol"},
				},
			},
		},
	}

	results, err := RunTransform(context.Background(), script, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Label != "Engineering / Alice" {
		t.Errorf("expected 'Engineering / Alice', got %q", results[0].Label)
	}
	if results[2].Label != "Design / Carol" {
		t.Errorf("expected 'Design / Carol', got %q", results[2].Label)
	}
}

func TestRunTransform_Timeout(t *testing.T) {
	script := `
function transform(data) {
	while(true) {} // infinite loop
	return [];
}
`
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := RunTransform(ctx, script, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunTransform_InvalidReturn(t *testing.T) {
	script := `
function transform(data) {
	return "not an array";
}
`
	_, err := RunTransform(context.Background(), script, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for non-array return")
	}
}

func TestRunTransform_MissingValueField(t *testing.T) {
	script := `
function transform(data) {
	return [{ label: "Test" }];
}
`
	_, err := RunTransform(context.Background(), script, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing value field")
	}
}

func TestRunTransform_SyntaxError(t *testing.T) {
	script := `
function transform(data) {
	return [{ label: "ok" value: 1 }]; // missing comma
}
`
	_, err := RunTransform(context.Background(), script, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestRunTransform_EmptyArray(t *testing.T) {
	script := `
function transform(data) {
	return [];
}
`
	results, err := RunTransform(context.Background(), script, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d items", len(results))
	}
}
