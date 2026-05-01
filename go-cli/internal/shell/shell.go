package shell

import (
	"fmt"
	"strings"
)

func Escape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func EnvLine(key string, value string) string {
	escaped := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
		"\n", "\\n",
	).Replace(value)
	return fmt.Sprintf("%s=\"%s\"", key, escaped)
}
