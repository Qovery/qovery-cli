//go:build testing

package promptuifactory

import (
	"fmt"
)

// PromptUiFactoryMock
type PromptUiFactoryMock struct {
	// Parameter to trigger an error
	forceError           map[string]bool
	expectedValueByLabel map[string]string
}

func NewPromptUiFactoryMock(
	forceError map[string]bool, // would use a Set but only a Map is available, so use bool as value
	expectedValueByLabel map[string]string,
) *PromptUiFactoryMock {
	return &PromptUiFactoryMock{
		forceError:           forceError,
		expectedValueByLabel: expectedValueByLabel,
	}
}

func (factory *PromptUiFactoryMock) RunPrompt(label string, defaultValue string) (string, error) {
	_, forceError := factory.forceError[label]
	if forceError {
		return "", fmt.Errorf("error for prompt '%s'", label)
	} else {
		var value, found = factory.expectedValueByLabel[label]
		if !found {
			return defaultValue, nil
		}
		return value, nil
	}
}
func (factory *PromptUiFactoryMock) RunSelect(label string, items []string) (int, string, error) {
	return factory.RunSelectWithSize(label, items, 5)
}
func (factory *PromptUiFactoryMock) RunSelectWithSize(label string, items []string, size int) (int, string, error) {
	return factory.RunSelectWithSizeAndSearcher(label, items, 5, func(string, int) bool { return true })
}

func (factory *PromptUiFactoryMock) RunSelectWithSizeAndSearcher(label string, items []string, size int, searcher func(string, int) bool) (int, string, error) {
	_, forceError := factory.forceError[label]
	if forceError {
		return -1, "", fmt.Errorf("error for select '%s'", label)
	} else {
		var value = factory.expectedValueByLabel[label]
		return 0, value, nil
	}
}
