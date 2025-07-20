package omit

import (
	"encoding/json"
)

func New[T any](value T) Omit[T] {
	return Omit[T]{
		Value: value,
		OK:    true,
	}
}

func NewZero[T any]() Omit[T] {
	return Omit[T]{
		OK: false,
	}
}

type Omit[T any] struct {
	Value T    `json:"value"`
	OK    bool `json:"ok"`
}

func (o Omit[T]) IsZero() bool {
	return !o.OK
}

func (o Omit[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Value)
}

func (o Omit[T]) UnmarshalJSON(data []byte) error {
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	o.Value = value
	o.OK = true

	return nil
}
