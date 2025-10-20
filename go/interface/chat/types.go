package chat

// Request represents a chat request from the user
type Request struct {
	Input   string
	Context map[string]interface{} // Additional context (e.g., session info)
}

// Response represents Wilson's response
type Response struct {
	Text      string
	ToolUsed  string // Empty if no tool was used
	Success   bool
	Error     error
	Artifacts []string // IDs of artifacts created
}

// NewRequest creates a new chat request
func NewRequest(input string) *Request {
	return &Request{
		Input:   input,
		Context: make(map[string]interface{}),
	}
}

// NewResponse creates a new response
func NewResponse(text string) *Response {
	return &Response{
		Text:    text,
		Success: true,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(err error) *Response {
	return &Response{
		Success: false,
		Error:   err,
	}
}
