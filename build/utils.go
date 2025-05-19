package main

import (
	"strings"
)

func mlnRecord(in map[string]string) string {
	var buf strings.Builder
	for k, v := range in {
		buf.WriteString(k)
		buf.WriteRune('=')
		buf.WriteString(v)
		buf.WriteRune('\n')
	}
	return buf.String()
}
