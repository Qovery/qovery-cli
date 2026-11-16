package pkg

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestClusterStatusColor(t *testing.T) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cases := map[string]*color.Color{
		"DEPLOYED":            green,
		"READY":               green,
		"RESTARTED":           green,
		"DEPLOYMENT_ERROR":    red,
		"BUILD_ERROR":         red,
		"INTERNAL_ERROR":      red,
		"INVALID_CREDENTIALS": red,
		"CANCELED":            yellow,
		"STOPPED":             yellow,
		"DELETED":             yellow,
	}
	for status, want := range cases {
		t.Run(status, func(t *testing.T) {
			assert.True(t, clusterStatusColor(status).Equals(want))
		})
	}
}
