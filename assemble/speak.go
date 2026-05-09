package assemble

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/organ"
)

var speakSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"response": {"type": "string", "description": "your response to the operator"}
	},
	"required": ["response"]
}`)

func speakCapability() organ.Func {
	return organ.Func{
		Name:        "dialog_speak",
		Description: "Respond to the operator. Use this to answer questions, greet, or provide information.",
		Schema:      speakSchema,
		Mode:        organ.ReadAction,
		Risk:        0,
		Source:      organ.BuiltIn,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct {
				Response string `json:"response"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return organ.TextResult(string(input)), nil
			}
			return organ.TextResult(args.Response), nil
		},
	}
}
