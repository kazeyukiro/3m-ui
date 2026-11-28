package converter

import (
	"encoding/base64"
)

// EncodeBase64 returns base64 string representation of the given bytes.
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
