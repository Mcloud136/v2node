package format

import (
	"strings"
)

func UserTag(tag string, uuid string) string {
	// 使用 strings.Builder 替代 fmt.Sprintf，减少内存分配
	var builder strings.Builder
	builder.Grow(len(tag) + 1 + len(uuid))
	builder.WriteString(tag)
	builder.WriteByte('|')
	builder.WriteString(uuid)
	return builder.String()
}
