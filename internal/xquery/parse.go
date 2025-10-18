package xquery

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-tools/internal/xstrconv"
)

func ParseTime(query url.Values, name string, defaultValue time.Time) time.Time {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func ParseBool(query url.Values, name string, defaultValue bool) bool {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := xstrconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func ParseInt(query url.Values, name string, defaultValue int) int {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func ParseString(query url.Values, name string, defaultValue string) string {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func ParseStringSlice(query url.Values, name string, defaultValue []string) []string {
	value := query.Get(name)
	if value == "" {
		return defaultValue
	}

	var result []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}
