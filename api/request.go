package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func NewRequest(method string, path string, parameters ...interface{}) *Request {
	r := Request{
		Method:     method,
		Path:       path,
		Parameters: parameters,
		token:      GetAuthorizationToken(),
	}
	return &r
}

type Request struct {
	Method        string
	Body          io.Reader
	Path          string
	Parameters    []interface{}
	URLParameters map[string]string
	token         string
}

func (r *Request) getUrl() string {
	u := fmt.Sprintf(RootURL+r.Path, r.Parameters...)
	if len(r.URLParameters) == 0 { // no url parameters, avoid to parse url
		return u
	}
	urlParsed, _ := url.Parse(u)
	q := urlParsed.Query()
	for name, value := range r.URLParameters {
		q.Set(name, value)
	}
	urlParsed.RawQuery = q.Encode()
	return urlParsed.String()
}

func (r *Request) SetJsonBody(body interface{}) *Request {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(body)
	r.Body = b
	return r
}

// build *http.Request
func (r *Request) getHttpRequest() (*http.Request, error) {
	req, err := http.NewRequest(r.Method, r.getUrl(), r.Body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerAuthorization, headerValueBearer+r.token)
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		req.Header.Set("content-type", "application/json")
	}
	return req, nil
}

func (r *Request) Do(response interface{}) error {
	req, err := r.getHttpRequest()
	if err != nil {
		return err
	}

	httpClient := http.Client{Timeout: Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	CheckHTTPResponse(resp)
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(response)
}
