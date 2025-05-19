package main

import (
	"encoding/json"
	"strings"
)

func mlnRecord(in map[string]any) (string, error) {
	var buf strings.Builder
	for k, v := range in {
		buf.WriteString(k)
		buf.WriteRune('=')
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}
