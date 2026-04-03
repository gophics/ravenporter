package cache

import (
	"encoding/base64"
	"net/url"
	"strings"
)

const (
	dataURIPrefix = "data:"
	dataURIBase64 = ";base64"
)

func isDataURI(value string) bool {
	return strings.HasPrefix(strings.ToLower(value), dataURIPrefix)
}

func decodeDataURI(value string) ([]byte, error) {
	prefix, payload, ok := strings.Cut(value, ",")
	if !ok {
		return nil, fmtErrorf("cache: malformed data URI")
	}
	if strings.HasSuffix(strings.ToLower(prefix), dataURIBase64) {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmtErrorf("cache: decode data URI: %w", err)
		}
		return data, nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return nil, fmtErrorf("cache: decode data URI: %w", err)
	}
	return []byte(decoded), nil
}
