package utils

import "strings"

func IsEmptyOrBlank(str string) bool {
	return len(strings.Trim(str, " ")) == 0
}
