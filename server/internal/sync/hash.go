package sync

import (
	"fmt"
	"strconv"
)

// ContentHash matches the plugin's contentHash for compatibility.
func ContentHash(s string) string {
	var h int32
	for i := 0; i < len(s); i++ {
		h = (h<<5 - h + int32(s[i])) | 0
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("sha256:%s%d", strconv.FormatInt(int64(h), 36), len(s))
}
