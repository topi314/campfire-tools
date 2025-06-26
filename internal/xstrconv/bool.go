package xstrconv

import (
	"strconv"
	"strings"
)

func ParseBool(str string) (bool, error) {
	switch strings.ToLower(str) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	default:
		return strconv.ParseBool(str)
	}
}
