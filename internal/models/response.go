package models

// Ответы ошибок API.
type ErrorResponse struct {
	Error string `json:"error"`
}

type PaginatedTasksResponse struct {
    Data   []Task `json:"data"`
    Limit  int    `json:"limit"`
    Offset int    `json:"offset"`
    Total  int    `json:"total"`
}