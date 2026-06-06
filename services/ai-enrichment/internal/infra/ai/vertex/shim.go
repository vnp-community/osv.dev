// shim.go — Method shims to satisfy port.LLMProvider interface.
// VertexGeminiAdapter uses Complete() naming; this adds GenerateStructured() wrapper.
package vertex

import "context"

// GenerateStructured satisfies port.LLMProvider by delegating to Complete.
func (a *VertexGeminiAdapter) GenerateStructured(ctx context.Context, prompt string, out interface{}) error {
	return a.Complete(ctx, prompt, out)
}
