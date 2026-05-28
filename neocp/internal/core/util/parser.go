package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseByteSize converts strings like "1GB", "100MB", "1TB", "unlimited" to MB (int64)
func ParseByteSize(input string) (int64, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "unlimited" || input == "-1" || input == "" {
		return -1, nil
	}

	re := regexp.MustCompile(`^(\d+)\s*(mb|gb|tb|m|g|t)?$`)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid size format: %s", input)
	}

	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := matches[2]
	switch unit {
	case "mb", "m":
		return val, nil
	case "gb", "g":
		return val * 1024, nil
	case "tb", "t":
		return val * 1024 * 1024, nil
	default:
		// Default to MB if no unit specified
		return val, nil
	}
}

// FormatByteSize converts MB (int64) back to human readable format
func FormatByteSize(mb int64) string {
	if mb == -1 {
		return "unlimited"
	}
	if mb >= 1024*1024 {
		return fmt.Sprintf("%.1fTB", float64(mb)/(1024*1024))
	}
	if mb >= 1024 {
		return fmt.Sprintf("%.1fGB", float64(mb)/1024)
	}
	return fmt.Sprintf("%dMB", mb)
}
