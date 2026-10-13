package ai

// ToolDefinition represents a schema for function calling by LLM chat clients.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a model-requested invocation of a specific tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// BookCitation records an exact paragraph reference for user verification.
type BookCitation struct {
	BookID         string  `json:"bookId"`
	BookTitle      string  `json:"bookTitle"`
	AuthorName     string  `json:"authorName,omitempty"`
	ChapterID      string  `json:"chapterId"`
	ChapterTitle   string  `json:"chapterTitle"`
	ChapterIndex   int     `json:"chapterIndex"`
	StartParagraph int     `json:"startParagraph"`
	EndParagraph   int     `json:"endParagraph"`
	Excerpt        string  `json:"excerpt"`
	CosineDistance float32 `json:"cosineDistance,omitempty"`
}

// ToolExecutionResult represents the internal output of executing a tool,
// including text for the model context and structured citations for client rendering.
type ToolExecutionResult struct {
	Content   string         `json:"content"`
	IsError   bool           `json:"isError,omitempty"`
	Citations []BookCitation `json:"citations,omitempty"`
}
