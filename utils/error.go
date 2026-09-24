package utils

import (
	"fmt"
)

type HttpResponseError struct {
	Code    int
	Message string
}

func (m *HttpResponseError) Error() string {
	return fmt.Sprintf("\nHTTP Response Code: %d\nError Message: %s", m.Code, m.Message)
}
