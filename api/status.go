package api

import (
	"github.com/fatih/color"
	"strings"
)

type Status struct {
	State                string `json:"state"`
	Code                 string `json:"code"`
	CodeMessage          string `json:"code_message"`
	Output               string `json:"output"`
	ProgressionInPercent int    `json:"progression_in_percent"`
}

func (s *Status) GetColoredCodeMessage() string {
	if strings.HasSuffix(s.Code, "_ERROR") {
		return color.RedString(s.CodeMessage)
	}

	if s.ProgressionInPercent != 100 {
		return color.YellowString(s.CodeMessage)
	}

	return color.GreenString(s.CodeMessage)
}
