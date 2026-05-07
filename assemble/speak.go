package assemble

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/capability"
)

var speakSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"response": {"type": "string", "description": "your response to the operator"}
	},
	"required": ["response"]
}`)

func speakCapability() capability.Capability {
	return capability.Capability{
		Name:        "speak",
		Description: "Respond to the operator. Use this to answer questions, greet, or provide information.",
		Schema:      speakSchema,
		Mode:        capability.ReadAction,
		Risk:        0,
		Source:      capability.BuiltIn,
		Execute: func(_ context.Context, input json.RawMessage) (capability.Result, error) {
			var args struct {
				Response string `json:"response"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return capability.TextResult(string(input)), nil
			}
			return capability.TextResult(args.Response), nil
		},
	}
}
