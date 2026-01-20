// ocr/types.go
package ocr

type OcrParams struct {
	Model             string  `json:"model"`              // e.g. "typhoon-ocr"
	TaskType          string  `json:"task_type"`          // e.g. "default"
	MaxTokens         int     `json:"max_tokens"`         // e.g. 16000
	Temperature       float64 `json:"temperature"`        // e.g. 0.1
	TopP              float64 `json:"top_p"`              // e.g. 0.6
	RepetitionPenalty float64 `json:"repetition_penalty"` // e.g. 1.2
	Pages             []int   `json:"pages,omitempty"`    // optional (for PDFs)
}

type OcrResponse struct {
	TotalPages      int         `json:"total_pages"`
	SuccessfulPages int         `json:"successful_pages"`
	FailedPages     int         `json:"failed_pages"`
	Results         []OcrResult `json:"results"`
	ProcessingTime  float64     `json:"processing_time"`
}

type OcrResult struct {
	Filename string      `json:"filename"`
	Success  bool        `json:"success"`
	Message  *OcrMessage `json:"message"`
	Error    interface{} `json:"error"`
	FileType string      `json:"file_type"`
	FileSize int64       `json:"file_size"`
	PageNum  *int        `json:"page_num"`
	BlobID   string      `json:"blob_id"`
	Duration float64     `json:"duration"`
}

type OcrMessage struct {
	ID                string      `json:"id"`
	Created           int64       `json:"created"`
	Model             string      `json:"model"`
	Object            string      `json:"object"`
	SystemFingerprint interface{} `json:"system_fingerprint"`
	Choices           []OcrChoice `json:"choices"`
	Usage             *OcrUsage   `json:"usage"`
	ServiceTier       interface{} `json:"service_tier"`
	PromptLogprobs    interface{} `json:"prompt_logprobs"`
	PromptTokenIDs    interface{} `json:"prompt_token_ids"`
	KVTransferParams  interface{} `json:"kv_transfer_params"`
}

type OcrChoice struct {
	FinishReason string         `json:"finish_reason"`
	Index        int            `json:"index"`
	Message      OcrChatMessage `json:"message"`
}

type OcrChatMessage struct {
	Content          string      `json:"content"`
	Role             string      `json:"role"`
	ToolCalls        interface{} `json:"tool_calls"`
	FunctionCall     interface{} `json:"function_call"`
	Refusal          interface{} `json:"refusal"`
	Annotations      interface{} `json:"annotations"`
	ReasoningContent interface{} `json:"reasoning_content"`
}

type OcrUsage struct {
	CompletionTokens        int         `json:"completion_tokens"`
	PromptTokens            int         `json:"prompt_tokens"`
	TotalTokens             int         `json:"total_tokens"`
	CompletionTokensDetails interface{} `json:"completion_tokens_details"`
	PromptTokensDetails     interface{} `json:"prompt_tokens_details"`
}
