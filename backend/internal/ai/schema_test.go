package ai

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"calisthenics/api/internal/plan"
)

// A hand-written schema has one failure mode worse than being rejected: being
// accepted while naming fields the Go struct does not have. The model then
// answers valid JSON, nothing errors, and the plan quietly comes back empty.
// So every object in both schemas is checked against the type it fills.

type schemaObject struct {
	Type                 string                     `json:"type"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Items                json.RawMessage            `json:"items"`
	Ref                  string                     `json:"$ref"`
	Defs                 map[string]json.RawMessage `json:"$defs"`
}

func parseObject(t *testing.T, raw json.RawMessage, where string) schemaObject {
	t.Helper()
	var obj schemaObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s: not valid JSON Schema: %v", where, err)
	}
	return obj
}

// resolve follows a $ref into the root's $defs; anything else is itself. The
// reference may sit at either level — key_drills is a $ref to a whole array,
// while progression is an array whose items are spelled out.
func resolve(t *testing.T, root schemaObject, raw json.RawMessage, where string) (schemaObject, string) {
	t.Helper()
	obj := parseObject(t, raw, where)
	if obj.Ref == "" {
		return obj, where
	}
	name := strings.TrimPrefix(obj.Ref, "#/$defs/")
	def, ok := root.Defs[name]
	if !ok {
		t.Fatalf("%s: $ref %q has no definition", where, obj.Ref)
	}
	return resolve(t, root, def, where+"->"+name)
}

// itemsOf is the object one element of an array must be.
func itemsOf(t *testing.T, root schemaObject, raw json.RawMessage, where string) schemaObject {
	t.Helper()
	arr, at := resolve(t, root, raw, where)
	if arr.Type != "array" || len(arr.Items) == 0 {
		t.Fatalf("%s: expected an array with items", at)
	}
	item, _ := resolve(t, root, arr.Items, at+".items")
	return item
}

// jsonFields is every name the Go type will actually read.
func jsonFields(v any) []string {
	var names []string
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	sort.Strings(names)
	return names
}

// checkObject asserts the schema describes exactly the fields the type reads,
// minus the ones the app fills in itself, and that it is strict.
func checkObject(t *testing.T, where string, obj schemaObject, target any, appFilled ...string) {
	t.Helper()

	if obj.AdditionalProperties == nil || *obj.AdditionalProperties {
		t.Errorf("%s: additionalProperties must be false for a strict schema", where)
	}

	var want []string
	skip := map[string]bool{}
	for _, name := range appFilled {
		skip[name] = true
	}
	for _, name := range jsonFields(target) {
		if !skip[name] {
			want = append(want, name)
		}
	}

	var have []string
	for name := range obj.Properties {
		have = append(have, name)
	}
	sort.Strings(have)

	if !reflect.DeepEqual(want, have) {
		t.Errorf("%s: schema describes %v, the struct reads %v", where, have, want)
	}

	// Strict schemas require every property; a missing one is a field the
	// model may silently omit.
	required := append([]string(nil), obj.Required...)
	sort.Strings(required)
	if !reflect.DeepEqual(have, required) {
		t.Errorf("%s: required is %v but properties are %v", where, required, have)
	}
}

func TestResearchSchemaMatchesTheStructItFills(t *testing.T) {
	root := parseObject(t, researchSchema, "researchSchema")

	// sources, searches_used, cached and researched_at are the app's to write:
	// the API reports which pages it retrieved, and asking a model for them is
	// asking it to invent citations.
	checkObject(t, "researchSchema", root, SkillResearch{},
		"sources", "searches_used", "cached", "researched_at")

	checkObject(t, "progression[]",
		itemsOf(t, root, root.Properties["progression"], "progression"), ResearchStage{})
	checkObject(t, "key_drills[]",
		itemsOf(t, root, root.Properties["key_drills"], "key_drills"), ResearchDrill{})
	checkObject(t, "accessories[]",
		itemsOf(t, root, root.Properties["accessories"], "accessories"), ResearchDrill{})
}

func TestPlanSchemaMatchesTheStructItFills(t *testing.T) {
	root := parseObject(t, planSchema, "planSchema")

	// method and research are provenance: the app knows which producer wrote
	// the plan and what it read, and the model does not get to claim either.
	checkObject(t, "planSchema", root, plan.Plan{}, "method", "research")

	sessions := itemsOf(t, root, root.Properties["sessions"], "sessions")
	checkObject(t, "sessions[]", sessions, plan.Session{})
	checkObject(t, "sessions[].blocks[]",
		itemsOf(t, root, sessions.Properties["blocks"], "sessions[].blocks"), plan.Block{})
	checkObject(t, "phases[]",
		itemsOf(t, root, root.Properties["phases"], "phases"), plan.Phase{})
}

// The transport only attaches a schema when there is one to attach; a prose
// turn must not carry an empty output_config.
func TestOutputConfigIsOnlyAttachedForAShapedAnswer(t *testing.T) {
	if cfg := outputConfigFor(turn{Prompt: "write me a paragraph"}); cfg != nil {
		t.Fatalf("a prose turn carried %+v", cfg)
	}
	cfg := outputConfigFor(turn{Prompt: "x", Schema: researchSchema})
	if cfg == nil || cfg.Format == nil {
		t.Fatal("a shaped turn carried no output_config")
	}
	if cfg.Format.Type != "json_schema" {
		t.Fatalf("format type = %q", cfg.Format.Type)
	}
	if !json.Valid(cfg.Format.Schema) {
		t.Fatal("the attached schema is not valid JSON")
	}
}
