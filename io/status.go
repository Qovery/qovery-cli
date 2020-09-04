package io

import (
	"github.com/fatih/color"
)

type Status struct {
	//State string `json:"state"`
	Kind string `json:"kind"`
	//CodeMessage          string `json:"code_message"`
	Message              string `json:"message"`
	ProgressionInPercent int    `json:"progression_in_percent"`
}

func (s *Status) GetKind() string {
	if s.Kind == "" {
		return "unknown"
	}

	return s.Kind
}

func (s *Status) IsError() bool {
	return s.Kind == "ERROR" || s.Kind == "FAILED"
}

func (s *Status) GetColoredCodeMessage() string {
	if s.Kind == "" {
		return color.RedString("unknown")
	}

	if s.IsError() {
		return s.Kind + " " + color.RedString(s.Message)
	}

	if s.ProgressionInPercent != 100 {
		return s.Kind + " " + color.YellowString(s.Message)
	}

	return s.Kind + " " + color.GreenString(s.Message)
}
