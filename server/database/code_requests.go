package database

import (
	"time"
)

type CodeRequest struct {
	ID        int       `db:"code_request_id"`
	CreatedAt time.Time `db:"code_request_created_at"`
}
