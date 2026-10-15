package request

type CreateSemanticModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RawSQL      string `json:"raw_sql"`
}

type UpdateSemanticModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RawSQL      string `json:"raw_sql"`
}
