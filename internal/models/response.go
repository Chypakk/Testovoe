package models

// ErrorResponse represents an error response
// @Description Error response structure
type ErrorResponse struct {
    Error string `json:"error"`
}

// PaginatedTasks represents a paginated response for tasks
// @Description Paginated response containing tasks list and pagination metadata
type PaginatedTasksResponse struct {
    Data   []Task `json:"data"`
    Limit  int    `json:"limit"`
    Offset int    `json:"offset"`
    Total  int    `json:"total"`
}