package main

import (
	"encoding/base64"
	"strings"
)

// Base64 url-variant encoding with padding stripped.
// Note, this is the same encoding format as JWT.
func Base64UrlEncode(s []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(s), "=")
}

// Base64 url-variant decoding with padding stripped.
// Note, this is the same encoding format as JWT.
func Base64UrlDecode(s string) ([]byte, error) {
	if l := len(s) % 4; l > 0 {
		s += strings.Repeat("=", 4-l)
	}
	return base64.URLEncoding.DecodeString(s)
}
