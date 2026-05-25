package core

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSizeToBytes converts human-readable strings like "1GB", "1TB", "500MB" or "unlimited" into bytes.
// Returns -1 for "unlimited".
func ParseSizeToBytes(sizeStr string) (int64, error) {
	s := strings.ToLower(strings.TrimSpace(sizeStr))
	if s == "unlimited" || s == "0" || s == "" {
		return -1, nil
	}

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(s, "tb") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "tb")
	} else if strings.HasSuffix(s, "gb") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "gb")
	} else if strings.HasSuffix(s, "mb") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(s, "mb")
	} else if strings.HasSuffix(s, "kb") {
		multiplier = 1024
		numStr = strings.TrimSuffix(s, "kb")
	} else if strings.HasSuffix(s, "b") {
		multiplier = 1
		numStr = strings.TrimSuffix(s, "b")
	} else {
		// Assume MB if no suffix is provided
		multiplier = 1024 * 1024
		numStr = s
	}

	val, err := strconv.ParseInt(strings.TrimSpace(numStr), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return val * multiplier, nil
}

// ParseQuantity converts strings like "100" or "unlimited" to an integer.
// Returns -1 for "unlimited".
func ParseQuantity(qStr string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(qStr))
	if s == "unlimited" || s == "" {
		return -1, nil
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity: %s", qStr)
	}
	return val, nil
}
