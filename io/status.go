package io

import (
	"github.com/fatih/color"
	"strings"
)

type Status struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (s *Status) GetKind() string {
	if s.Kind == "" {
		return "unknown"
	}

	return s.Kind
}

func (s *Status) IsOk() bool {
	return s.IsDone() || s.Kind == "DELETED"
}

func (s *Status) IsDone() bool {
	return s.Kind == "DONE"
}

func (s *Status) IsWaiting() bool {
	return s.Kind == "WAITING"
}

func (s *Status) IsFailed() bool {
	return s.Kind == "FAILED"
}

func (s *Status) GetColoredCodeMessage() string {
	if s.Kind == "" {
		return color.RedString("unknown")
	}

	if s.IsDone() {
		return color.GreenString(strings.ToLower(s.Kind))
	} else if s.IsFailed() {
		return color.RedString(strings.ToLower(s.Kind))
	}

	// running and other states are yellow
	return color.YellowString(strings.ToLower(s.Kind))
}
