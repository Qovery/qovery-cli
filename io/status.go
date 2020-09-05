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
	return s.IsRunning() || s.Kind == "DONE" || s.Kind == "DELETED"
}

func (s *Status) IsRunning() bool {
	return s.Kind == "RUNNING"
}

func (s *Status) IsWaiting() bool {
	return s.Kind == "WAITING"
}

func (s *Status) GetColoredCodeMessage() string {
	if s.Kind == "" {
		return color.RedString("unknown")
	}

	if s.IsRunning() || s.Kind == "DONE" || s.Kind == "DELETED" {
		return color.GreenString(strings.ToLower(s.Kind))
	} else if s.Kind == "FAILED" || s.Kind == "ERROR" {
		return color.RedString(strings.ToLower(s.Kind))
	}

	return color.YellowString(strings.ToLower(s.Kind))
}
