// shim.go — Method shims to satisfy port.LLMProvider interface.
// OpenAI Client uses Complete() naming; this adds GenerateStructured() wrapper.
package openai

import "context"

// GenerateStructured satisfies port.LLMProvider by delegating to Complete.
func (c *Client) GenerateStructured(ctx context.Context, prompt string, out interface{}) error {
	return c.Complete(ctx, prompt, out)
}
