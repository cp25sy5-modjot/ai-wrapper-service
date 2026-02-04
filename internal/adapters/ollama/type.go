package ollama

type AIRequest struct {
	Model   string     `json:"model"`
	Prompt  string     `json:"prompt"`
	Stream  bool       `json:"stream"`
	Format  string     `json:"format"`
	Options *AIOptions `json:"options,omitempty"`
}

type AIOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}
