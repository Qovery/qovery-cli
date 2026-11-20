//! Port of `pkg/promptuifactory`: prompts injected into the services so that they can be mocked in tests.
//!
//! `PromptUiFactoryImpl` uses `qovery_core::select`: promptui byte-identical rendering and stdin consumption
//! when stdin is not a terminal, `inquire` on a terminal.

#[cfg(test)]
use std::collections::{HashMap, HashSet};

/// `promptuifactory.PromptUiFactory`.
pub trait PromptUiFactory {
    fn run_prompt(&self, label: &str, default_value: &str) -> Result<String, String>;
    fn run_select(&self, label: &str, items: &[String]) -> Result<(usize, String), String> {
        self.run_select_with_size(label, items, 5)
    }
    fn run_select_with_size(
        &self,
        label: &str,
        items: &[String],
        size: usize,
    ) -> Result<(usize, String), String>;
    fn run_select_with_size_and_searcher(
        &self,
        label: &str,
        items: &[String],
        size: usize,
        searcher: &dyn Fn(&str, usize) -> bool,
    ) -> Result<(usize, String), String>;
}

/// `promptuifactory.PromptUiFactoryImpl` (promptui byte-identical on a non-terminal stdin, see `qovery_core::select`).
pub struct PromptUiFactoryImpl;

impl PromptUiFactory for PromptUiFactoryImpl {
    fn run_prompt(&self, label: &str, default_value: &str) -> Result<String, String> {
        qovery_core::select::prompt_text_with_default(label, default_value)
    }

    fn run_select_with_size(
        &self,
        label: &str,
        items: &[String],
        size: usize,
    ) -> Result<(usize, String), String> {
        qovery_core::select::prompt_select_with_label(label, items, Some(size))
    }

    fn run_select_with_size_and_searcher(
        &self,
        label: &str,
        items: &[String],
        size: usize,
        searcher: &dyn Fn(&str, usize) -> bool,
    ) -> Result<(usize, String), String> {
        qovery_core::select::prompt_select_with_searcher(label, items, size, searcher)
    }
}

/// `promptuifactory.PromptUiFactoryMock`: prompts answered by label, errors forced by label.
#[cfg(test)]
pub struct PromptUiFactoryMock {
    force_error: HashSet<String>,
    expected_value_by_label: HashMap<String, String>,
}

#[cfg(test)]
impl PromptUiFactoryMock {
    pub fn new(force_error: &[&str], expected: &[(&str, &str)]) -> Self {
        PromptUiFactoryMock {
            force_error: force_error.iter().map(|s| s.to_string()).collect(),
            expected_value_by_label: expected
                .iter()
                .map(|(k, v)| (k.to_string(), v.to_string()))
                .collect(),
        }
    }
}

#[cfg(test)]
impl PromptUiFactory for PromptUiFactoryMock {
    fn run_prompt(&self, label: &str, default_value: &str) -> Result<String, String> {
        if self.force_error.contains(label) {
            return Err(format!("error for prompt '{label}'"));
        }
        Ok(self
            .expected_value_by_label
            .get(label)
            .cloned()
            .unwrap_or_else(|| default_value.to_string()))
    }

    fn run_select_with_size(
        &self,
        label: &str,
        items: &[String],
        _size: usize,
    ) -> Result<(usize, String), String> {
        self.run_select_with_size_and_searcher(label, items, 5, &|_, _| true)
    }

    fn run_select_with_size_and_searcher(
        &self,
        label: &str,
        _items: &[String],
        _size: usize,
        _searcher: &dyn Fn(&str, usize) -> bool,
    ) -> Result<(usize, String), String> {
        if self.force_error.contains(label) {
            return Err(format!("error for select '{label}'"));
        }
        Ok((
            0,
            self.expected_value_by_label
                .get(label)
                .cloned()
                .unwrap_or_default(),
        ))
    }
}

/// `utils.Validate(actionType)`.
pub use qovery_core::select::validate;

/// promptui `Select{Label, Items}.Run()` (default size 5), used directly by `cluster upgrade`.
pub fn select(label: &str, items: &[&str]) -> Result<String, String> {
    let items: Vec<String> = items.iter().map(|s| s.to_string()).collect();
    PromptUiFactoryImpl
        .run_select_with_size(label, &items, 5)
        .map(|(_, v)| v)
}
