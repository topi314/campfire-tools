package xrand

import (
	"math/rand/v2"
)

const letters = "0123456789"

func RandCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
