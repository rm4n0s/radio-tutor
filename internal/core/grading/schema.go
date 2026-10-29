package grading

// ReplySchema returns the JSON Schema of the model's reply (SPEC §11.5) as a
// value ready to be encoded into the request's response_format. Every call
// returns a new map, so the caller may change it.
func ReplySchema() map[string]any {
	verdicts := make([]string, 0, 5)
	for _, v := range Verdicts() {
		verdicts = append(verdicts, string(v))
	}
	str := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"page_notes", "results"},
		"properties": map[string]any{
			"page_notes": str("Notes for the parent about the photos as a whole; empty when there is nothing to say."),
			"results": map[string]any{
				"type":        "array",
				"description": "Exactly one entry for each exercise in the cassette block, in the same order.",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"exercise_id", "verdict", "transcription", "feedback_for_child", "notes_for_parent"},
					"properties": map[string]any{
						"exercise_id": str("The id of the exercise, exactly as written in the cassette block."),
						"verdict": map[string]any{
							"type":        "string",
							"enum":        verdicts,
							"description": "The judgement of the solution.",
						},
						"transcription":      str("What you read for this exercise, as the child wrote it."),
						"feedback_for_child": str("Short, warm feedback for the child, in the cassette's language."),
						"notes_for_parent":   str("What the child wrote and why the verdict, in the cassette's language."),
					},
				},
			},
		},
	}
}
