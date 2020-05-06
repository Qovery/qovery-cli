package io

import (
	"os"
	"strings"
)

func GetAbsoluteParentPath(path string) string {
	if path == "" {
		return ""
	}

	s := strings.Split(path, string(os.PathSeparator))
	return strings.Join(s[0:len(s)-1], string(os.PathSeparator))
}
