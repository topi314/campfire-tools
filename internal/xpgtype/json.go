package xpgtype

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

var (
	_ sql.Scanner   = (*JSON[any])(nil)
	_ driver.Valuer = (*JSON[any])(nil)
)

func NewJSON[T any](v T) JSON[T] {
	return JSON[T]{V: v}
}

type JSON[T any] struct {
	V T
}

func (j JSON[T]) Value() (driver.Value, error) {
	return json.Marshal(j.V)
}

func (j *JSON[T]) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		return errors.New("expected []byte for JSON scan")
	}

	return json.Unmarshal(b, &j.V)
}
