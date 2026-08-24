package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandExecutionPropertiesContainOnlySafeStructuredValues(t *testing.T) {
	execution := CommandExecutionProperties{
		AttemptID:        "0198dc0e-b7ab-7b91-8c42-2169f9302572",
		WorkflowType:     "demo_installation",
		Implementation:   "engine_v2",
		Result:           "failed",
		DurationMillis:   1234,
		ErrorCode:        "UNKNOWN_FAILURE",
		SafeErrorMessage: "The demo installation failed.",
	}

	properties := commandExecutionPostHogProperties(execution)

	assert.Equal(t, "0198dc0e-b7ab-7b91-8c42-2169f9302572", properties["attempt_id"])
	assert.Equal(t, "demo_installation", properties["workflow_type"])
	assert.Equal(t, "engine_v2", properties["implementation"])
	assert.Equal(t, "failed", properties["result"])
	assert.Equal(t, int64(1234), properties["duration_ms"])
	assert.Equal(t, "UNKNOWN_FAILURE", properties["error_code"])
	assert.Equal(t, "The demo installation failed.", properties["safe_error_message"])
	assert.NotContains(t, properties, "stdout")
	assert.NotContains(t, properties, "stderr")
}
