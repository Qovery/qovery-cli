package utils

import (
	"fmt"
	"io"
	"net/http"
)

type HttpResponseError struct {
	Code    int
	Message string
}

func toHttpResponseError(response *http.Response) *HttpResponseError {
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	return &HttpResponseError{
		Code:    response.StatusCode,
		Message: string(body),
	}
}

func (m *HttpResponseError) Error() string {
	return fmt.Sprintf("\nHTTP Response Code: %d\nError Message: %s", m.Code, m.Message)
}
