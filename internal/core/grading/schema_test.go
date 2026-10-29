package grading

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// jsonFields returns the JSON names of the fields of a struct type.
func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		out = append(out, strings.Split(t.Field(i).Tag.Get("json"), ",")[0])
	}
	sort.Strings(out)
	return out
}

func sortedStrings(t *testing.T, v any) []string {
	t.Helper()
	s, ok := v.([]string)
	if !ok {
		t.Fatalf("%T is not []string", v)
	}
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestReplySchema(t *testing.T) {
	schema := ReplySchema()
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Errorf("top level: %v", schema)
	}
	top := sortedStrings(t, schema["required"])
	if !reflect.DeepEqual(top, jsonFields(reflect.TypeFor[Reply]())) {
		t.Errorf("required = %v, Reply fields = %v", top, jsonFields(reflect.TypeFor[Reply]()))
	}
	props := schema["properties"].(map[string]any)
	if len(props) != len(top) {
		t.Errorf("%d properties for %d required fields", len(props), len(top))
	}

	results := props["results"].(map[string]any)
	if results["type"] != "array" {
		t.Errorf("results type = %v", results["type"])
	}
	item := results["items"].(map[string]any)
	if item["type"] != "object" || item["additionalProperties"] != false {
		t.Errorf("item: %v", item)
	}
	required := sortedStrings(t, item["required"])
	if want := jsonFields(reflect.TypeFor[ExerciseResult]()); !reflect.DeepEqual(required, want) {
		t.Errorf("item required = %v, ExerciseResult fields = %v", required, want)
	}
	itemProps := item["properties"].(map[string]any)
	if len(itemProps) != len(required) {
		t.Errorf("%d item properties for %d required", len(itemProps), len(required))
	}
	for name, p := range itemProps {
		if p.(map[string]any)["type"] != "string" {
			t.Errorf("%s is not a string", name)
		}
	}
	verdict := itemProps["verdict"].(map[string]any)
	var enum []string
	for _, v := range Verdicts() {
		enum = append(enum, string(v))
	}
	if got := verdict["enum"].([]string); !reflect.DeepEqual(got, enum) {
		t.Errorf("verdict enum = %v", got)
	}
}

func TestReplySchemaEncodesAndIsFresh(t *testing.T) {
	b, err := json.Marshal(ReplySchema())
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back["type"] != "object" {
		t.Errorf("round trip: %v", back)
	}
	a := ReplySchema()
	a["required"] = nil
	if ReplySchema()["required"] == nil {
		t.Error("ReplySchema shares state between calls")
	}
}
