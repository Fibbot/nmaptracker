package web

import (
	"strconv"
	"strings"
)

func parseInt(value string, fallback int) int {
	val, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || val <= 0 {
		return fallback
	}
	return val
}
