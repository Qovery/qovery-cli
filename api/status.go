package api

type Status struct {
	State       string `json:"state"`
	Code        string `json:"code"`
	CodeMessage string `json:"code_message"`
	Output      string `json:"output"`
}
