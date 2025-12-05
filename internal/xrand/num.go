package xrand

import (
	"math/rand/v2"
)

const (
	numbers = "0123456789"
	letters = "abcdefghijklmnopqrstuvwxyz123456789"
)

func NumberCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = numbers[rand.IntN(len(numbers))]
	}
	return string(b)
}

func Code(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
