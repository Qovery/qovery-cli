package api

import (
	"github.com/fatih/color"
	"strings"
)

type Status struct {
	State                string `json:"state"`
	Code                 int    `json:"code"`
	CodeMessage          string `json:"code_message"`
	Output               string `json:"output"`
	ProgressionInPercent int    `json:"progression_in_percent"`
}

func (s *Status) GetState() string {
	if s.State == "" {
		return "unknown"
	}

	return s.State
}

func (s *Status) IsError() bool {
	if strings.HasSuffix(s.State, "_ERROR") {
		return true
	}

	return false
}

func (s *Status) GetColoredCodeMessage() string {
	if s.CodeMessage == "" {
		return color.RedString("unknown")
	}

	if s.IsError() {
		return color.RedString(s.CodeMessage)
	}

	if s.ProgressionInPercent != 100 {
		return color.YellowString(s.CodeMessage)
	}

	return color.GreenString(s.CodeMessage)
}
