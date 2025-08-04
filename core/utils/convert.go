// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func StringToInt(s string) (int, bool) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return i, true
}

func StringToBool(s string) (bool, bool) {
	s = strings.ToLower(s)
	switch s {
	case "true", "yes", "1":
		return true, true
	case "false", "no", "0":
		return false, true
	default:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return false, false
		}
		return b, true
	}
}

func StringToFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func StringToSlice(s string, sep string) ([]string, bool) {
	if s == "" {
		return []string{}, true
	}
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		firstChar := s[0]
		lastChar := s[len(s)-1]
		if (firstChar == '[' && lastChar == ']') || (firstChar == '(' && lastChar == ')') || (firstChar == '{' && lastChar == '}') {
			s = s[1 : len(s)-1]
		}
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, true
}

func SliceToString(slice []string, sep string) string {
	return strings.Join(slice, sep)
}

func StringToStringSlice(s string) ([]string, bool) {
	if s == "" {
		return []string{}, true
	}
	s = strings.TrimSpace(s)

	if len(s) >= 2 {
		firstChar := s[0]
		lastChar := s[len(s)-1]
		if (firstChar == '[' && lastChar == ']') || (firstChar == '(' && lastChar == ')') || (firstChar == '{' && lastChar == '}') {
			s = s[1 : len(s)-1]
		}
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]

		var result []string
		var current string
		var inQuotes bool
		var escaped bool

		for i := 0; i < len(s); i++ {
			c := s[i]
			if escaped {
				current += string(c)
				escaped = false
				continue
			}

			if c == '\\' {
				escaped = true
				continue
			}

			if c == '"' {
				inQuotes = !inQuotes
				continue
			}

			if c == ',' && !inQuotes {
				result = append(result, current)
				current = ""
				continue
			}

			current += string(c)
		}

		if current != "" {
			result = append(result, current)
		}

		return result, true
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, true
}

func AnyToType(v any, targetType string) (string, bool) {
	targetType = strings.ToLower(targetType)
	switch targetType {
	case "string":
		return fmt.Sprintf("%v", v), true
	case "int":
		if i, ok := v.(int); ok {
			return strconv.Itoa(i), true
		}
		switch v := v.(type) {
		case int8, int16, int32, int64:
			return fmt.Sprintf("%d", v), true
		case uint, uint8, uint16, uint32, uint64:
			return fmt.Sprintf("%d", v), true
		case float32, float64:
			return fmt.Sprintf("%d", int(v.(float64))), true
		case string:
			if i, ok := StringToInt(v); ok {
				return strconv.Itoa(i), true
			}
		}
	case "float":
		if f, ok := v.(float64); ok {
			return strconv.FormatFloat(f, 'f', -1, 64), true
		}
		switch v := v.(type) {
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%f", v), true
		case uint, uint8, uint16, uint32, uint64:
			return fmt.Sprintf("%f", v), true
		case float32:
			return strconv.FormatFloat(float64(v), 'f', -1, 64), true
		case string:
			if f, ok := StringToFloat(v); ok {
				return strconv.FormatFloat(f, 'f', -1, 64), true
			}
		}
	case "bool":
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b), true
		}
		switch v := v.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return strconv.FormatBool(v.(int) != 0), true
		case float32, float64:
			return strconv.FormatBool(v.(float64) != 0), true
		case string:
			if b, ok := StringToBool(v); ok {
				return strconv.FormatBool(b), true
			}
		}
	}
	return "", false
}
