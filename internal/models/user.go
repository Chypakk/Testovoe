package models

import (
	"time"
)

// User represents a user in the system
// @Description User account information
type User struct {
	ID        uint      `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RegisterRequest represents the request for user registration
// @Description User registration data
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Name     string `json:"name" validate:"required"`
}

// LoginRequest represents the request for user login
// @Description User login credentials
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the response after successful login
// @Response containing JWT token and user info
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
