package main

import (
	"strconv"
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

func arguments(argName string, in map[string]string, prefix, suffix string) string {
	var buf strings.Builder
	for k, v := range in {
		if buf.Len() > 0 {
			buf.WriteRune(' ')
		}
		buf.WriteString("--")
		buf.WriteString(argName)
		buf.WriteString(" \"")
		buf.WriteString(prefix + k + suffix)
		buf.WriteRune('=')
		vQt := strconv.Quote(v)
		buf.WriteString(vQt[1 : len(vQt)-1])
		buf.WriteRune('"')
	}
	return buf.String()
}
