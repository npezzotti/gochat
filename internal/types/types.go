package types

import "time"

type User struct {
	Id           int       `json:"id"`
	Username     string    `json:"username"`
	EmailAddress string    `json:"email_address,omitempty"`
	Password     string    `json:"-"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}
