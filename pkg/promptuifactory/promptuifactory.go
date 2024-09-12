package promptuifactory

import "github.com/manifoldco/promptui"

// PromptUiFactory Used to generate necessary prompts injected into services
// The purpose is to be able to mock this Factory to be used in Unit Tests
type PromptUiFactory interface {
	RunPrompt(label string, defaultValue string) (string, error)
	RunSelect(label string, items []string) (int, string, error)
	RunSelectWithSize(label string, items []string, size int) (int, string, error)
	RunSelectWithSizeAndSearcher(label string, items []string, size int, searcher func(string, int) bool) (int, string, error)
}

type PromptUiFactoryImpl struct{}

func (factory *PromptUiFactoryImpl) RunPrompt(label string, defaultValue string) (string, error) {
	return (&promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}).Run()
}
func (factory *PromptUiFactoryImpl) RunSelect(label string, items []string) (int, string, error) {
	return factory.RunSelectWithSize(label, items, 5)
}
func (factory *PromptUiFactoryImpl) RunSelectWithSize(label string, items []string, size int) (int, string, error) {
	return (&promptui.Select{
		Label: label,
		Items: items,
		Size:  size,
	}).Run()
}

func (factory *PromptUiFactoryImpl) RunSelectWithSizeAndSearcher(label string, items []string, size int, searcher func(string, int) bool) (int, string, error) {
	return (&promptui.Select{
		Label:             label,
		Items:             items,
		Size:              size,
		Searcher:          searcher,
		StartInSearchMode: true,
	}).Run()
}