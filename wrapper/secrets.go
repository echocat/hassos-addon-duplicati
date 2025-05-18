package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

func generateSecretString() (string, error) {
	buf := make([]byte, 20)
	if n, err := rand.Reader.Read(buf); err != nil {
		return "", fmt.Errorf("cannot create new secret from random generator: %w", err)
	} else if n < len(buf) {
		return "", fmt.Errorf("cannot create new secret from random generator: read too few bytes (%d < %d)", n, len(buf))
	}

	result := base64.RawStdEncoding.EncodeToString(buf)
	result = strings.ReplaceAll(result, "+", "z")
	result = strings.ReplaceAll(result, "/", "0")
	return result, nil
}
