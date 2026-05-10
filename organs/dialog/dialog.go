package dialog

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/organ"
)

var schema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"response": {"type": "string", "description": "your response to the operator"}
	},
	"required": ["response"]
}`)

type Controller struct{}

var _ organ.Controller = (*Controller)(nil)

func New() *Controller { return &Controller{} }

func (*Controller) Name() string              { return "dialog" }
func (*Controller) Description() string       { return "Respond to the operator. Use this to answer questions, greet, or provide information." }
func (*Controller) Schema() json.RawMessage   { return schema }
func (*Controller) IsResponse() bool          { return true }

func (*Controller) Handle(_ context.Context, input json.RawMessage) (organ.Result, error) {
	var args struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return organ.TextResult(string(input)), nil
	}
	return organ.TextResult(args.Response), nil
}
