package ai

import "encoding/json"

// The shapes the model must answer in, as JSON Schema.
//
// Asking for JSON in the prompt is not the same as getting it. A model writing
// prose about pull-ups will happily put a quoted phrase inside a string —
// `more than lack of "back strength."` — and the document is then invalid,
// after the whole call has been paid for. That is not a rare edge: it is what
// happened on the first real run against Anthropic, and it cost a full
// research turn.
//
// So the shape is sent with the request. Where the provider supports it the
// API constrains generation and the failure cannot occur; where it does not,
// the schema is still what the repair turn is asked to satisfy.
//
// Only the fields the *model* fills are described. Sources, cache stamps and
// provenance are the app's to write, and a schema that demanded them would be
// asking the model to invent them.

// Every object in these schemas sets additionalProperties:false and lists
// every property as required, which is what a strict schema means. Fields that
// are optional in Go are required here and answered with "" or [] — the
// omitempty tags only ever affected what this app writes out, never what it
// reads in.

var researchSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["skill", "summary", "prerequisites", "progression", "key_drills",
               "accessories", "weekly_structure", "volume_guidance",
               "common_mistakes", "injury_risks"],
  "properties": {
    "skill": {"type": "string"},
    "summary": {"type": "string"},
    "prerequisites": {"type": "array", "items": {"type": "string"}},
    "progression": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["stage", "exercise_slugs", "standard", "typical_weeks"],
        "properties": {
          "stage": {"type": "string"},
          "exercise_slugs": {"type": "array", "items": {"type": "string"}},
          "standard": {"type": "string"},
          "typical_weeks": {"type": "string"}
        }
      }
    },
    "key_drills": {"$ref": "#/$defs/drills"},
    "accessories": {"$ref": "#/$defs/drills"},
    "weekly_structure": {"type": "string"},
    "volume_guidance": {"type": "string"},
    "common_mistakes": {"type": "array", "items": {"type": "string"}},
    "injury_risks": {"type": "array", "items": {"type": "string"}}
  },
  "$defs": {
    "drills": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["exercise_slug", "role", "dosage"],
        "properties": {
          "exercise_slug": {"type": "string"},
          "role": {"type": "string"},
          "dosage": {"type": "string"}
        }
      }
    }
  }
}`)

var planSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["title", "summary", "weeks", "restrictions", "phases",
               "progression_rules", "test", "notes", "sessions"],
  "properties": {
    "title": {"type": "string"},
    "summary": {"type": "string"},
    "weeks": {"type": "integer"},
    "restrictions": {"type": "array", "items": {"type": "string"}},
    "phases": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["weeks", "name", "aim"],
        "properties": {
          "weeks": {"type": "string"},
          "name": {"type": "string"},
          "aim": {"type": "string"}
        }
      }
    },
    "progression_rules": {"type": "array", "items": {"type": "string"}},
    "test": {"type": "string"},
    "notes": {"type": "array", "items": {"type": "string"}},
    "sessions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["week", "day_of_week", "title", "focus", "load",
                     "duration_minutes", "warmup_protocols", "blocks", "cooldown"],
        "properties": {
          "week": {"type": "integer"},
          "day_of_week": {"type": "integer"},
          "title": {"type": "string"},
          "focus": {"type": "string"},
          "load": {"type": "string"},
          "duration_minutes": {"type": "integer"},
          "warmup_protocols": {"type": "array", "items": {"type": "string"}},
          "cooldown": {"type": "string"},
          "blocks": {
            "type": "array",
            "items": {
              "type": "object",
              "additionalProperties": false,
              "required": ["exercise_slug", "intent", "sets", "prescription",
                           "intensity", "tempo", "rest_seconds", "progression", "notes"],
              "properties": {
                "exercise_slug": {"type": "string"},
                "intent": {"type": "string"},
                "sets": {"type": "integer"},
                "prescription": {"type": "string"},
                "intensity": {"type": "string"},
                "tempo": {"type": "string"},
                "rest_seconds": {"type": "integer"},
                "progression": {"type": "string"},
                "notes": {"type": "string"}
              }
            }
          }
        }
      }
    }
  }
}`)
