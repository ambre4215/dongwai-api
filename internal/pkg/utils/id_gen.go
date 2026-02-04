package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
)

func GenerateID(pre string, keys ...string) string {
	h := md5.New()
	for _, key := range keys {
		io.WriteString(h, key)
	}
	hash := hex.EncodeToString(h.Sum(nil))[:16]
	return fmt.Sprintf("%s%s", pre, hash)
}
