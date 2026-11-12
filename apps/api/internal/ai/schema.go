package ai

import (
	"bytes"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"slices"
)

const SchemaVersion = "composer.v3"

type schema = map[string]any

func object(properties schema) schema {
	required := []string{}
	for key := range properties {
		required = append(required, key)
	}
	slices.Sort(required)
	return schema{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func text() schema                        { return schema{"type": "string"} }
func array(item schema) schema            { return schema{"type": "array", "items": item} }
func enumeration(values ...string) schema { return schema{"type": "string", "enum": values} }
func reference(name string) schema        { return schema{"$ref": "#/$defs/" + name} }
func nullable(value schema) schema        { return schema{"anyOf": []any{value, schema{"type": "null"}}} }
func fallbacks() schema                   { return array(object(schema{"path": text(), "value": text()})) }

// Schema is shared by Claude and OpenAI. Conditions are explicitly unrolled to
// two group levels because Claude does not accept recursive JSON Schemas. The
// deterministic runtime and human editor support up to eight levels.
func Schema(kind string) schema {
	if kind == "explain" {
		return object(schema{"explanation": text(), "warnings": array(text())})
	}
	message := object(schema{"subject": text(), "preheader": text(), "html": text(), "text": text(), "fallbacks": fallbacks(), "explanation": text(), "warnings": array(text())})
	if kind == "message" {
		return message
	}
	scalar := schema{"type": []string{"string", "number", "boolean", "null"}}
	leaf := object(schema{"path": text(), "operator": enumeration("equals", "not_equals", "greater_than", "less_than", "contains", "exists", "before", "after"), "value": reference("Scalar")})
	defs := schema{"Scalar": scalar, "Leaf": leaf}
	for level := 1; level <= 2; level++ {
		previous := "Leaf"
		if level == 2 {
			previous = "Rule1"
		}
		name := "Rule1"
		if level == 2 {
			name = "Rule2"
		}
		defs[name] = schema{"anyOf": []any{reference("Leaf"), object(schema{"logic": enumeration("and", "or"), "rules": array(reference(previous))})}}
	}
	defs["MaybeRule"] = nullable(reference("Rule2"))
	configs := schema{
		"event_trigger":    object(schema{"event_name": text(), "filters": reference("MaybeRule")}),
		"wait":             object(schema{"duration_seconds": schema{"type": "integer"}, "timezone": text()}),
		"wait_event":       object(schema{"event_name": text(), "timeout_seconds": schema{"type": "integer"}, "filters": reference("MaybeRule")}),
		"condition":        object(schema{"rule": reference("Rule2")}),
		"email":            object(schema{"subject": text(), "preheader": text(), "html": text(), "text": text(), "fallbacks": fallbacks()}),
		"sms":              object(schema{"text": text(), "fallbacks": fallbacks()}),
		"update_attribute": object(schema{"attribute": text(), "operation": enumeration("set", "increment", "unset"), "value": reference("Scalar")}),
		"exit":             object(schema{"reason": enumeration("completed", "converted", "condition_not_met", "manually_cancelled")}),
	}
	variants := []any{}
	for _, kind := range []string{"event_trigger", "wait", "wait_event", "condition", "email", "sms", "update_attribute", "exit"} {
		variants = append(variants, object(schema{"id": text(), "type": enumeration(kind), "label": text(), "position": object(schema{"x": schema{"type": "number"}, "y": schema{"type": "number"}}), "config": configs[kind]}))
	}
	flow := object(schema{"schema_version": schema{"type": "integer", "enum": []int{1}}, "nodes": array(schema{"anyOf": variants}), "edges": array(object(schema{"id": text(), "source": text(), "target": text(), "port": enumeration("started", "completed", "matched", "timeout", "true", "false", "next")})), "global_exit": nullable(object(schema{"event_name": text(), "reason": enumeration("converted", "completed", "condition_not_met", "manually_cancelled"), "filters": reference("MaybeRule")})), "reentry": nullable(object(schema{"mode": enumeration("once", "after_completion", "cooldown"), "cooldown_seconds": schema{"type": "integer"}}))})
	root := object(schema{"name": text(), "description": text(), "workflow": flow, "explanation": text(), "warnings": array(text())})
	root["$defs"] = defs
	return root
}
func compile(kind string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	url := "https://journey.local/schemas/" + kind + ".json"
	raw, _ := json.Marshal(Schema(kind))
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if err := compiler.AddResource(url, document); err != nil {
		return nil, err
	}
	return compiler.Compile(url)
}
