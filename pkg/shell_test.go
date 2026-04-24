package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleLocalConsoleControlInput_WithCtrlC(t *testing.T) {
	data, shouldExit := handleLocalConsoleControlInput([]byte{'a', 3, 'b'}, false)

	assert.True(t, shouldExit)
	assert.Nil(t, data)
}

func TestHandleLocalConsoleControlInput_WithoutCtrlC(t *testing.T) {
	input := []byte("hello")

	data, shouldExit := handleLocalConsoleControlInput(input, false)

	assert.False(t, shouldExit)
	assert.Equal(t, input, data)
}

func TestHandleLocalConsoleControlInput_WithCtrlCAndActiveShell(t *testing.T) {
	input := []byte{'a', 3, 'b'}

	data, shouldExit := handleLocalConsoleControlInput(input, true)

	assert.False(t, shouldExit)
	assert.Equal(t, input, data)
}
