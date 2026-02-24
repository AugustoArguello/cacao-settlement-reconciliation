package response

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Field   string      `json:"field,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	TotalCount int         `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

type BatchIngestionResponse struct {
	Ingested          int              `json:"ingested"`
	DuplicatesSkipped int              `json:"duplicates_skipped"`
	Errors            []IngestionError `json:"errors,omitempty"`
}

type IngestionError struct {
	Index   int    `json:"index"`
	ID      string `json:"id"`
	Message string `json:"message"`
}
