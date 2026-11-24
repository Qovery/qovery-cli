//! Hooks run right before the process exits.
//!
//! `std::process::exit` (Go `os.Exit`) skips destructors: resources that must not outlive the CLI, like the
//! SSH tunnel of the bastion, register a hook here, and every exit path of the CLI goes through [`exit`].

use std::sync::Mutex;

type Hook = Box<dyn FnOnce() + Send>;

static HOOKS: Mutex<Vec<Hook>> = Mutex::new(Vec::new());

/// Register a hook run by [`exit`] (the last registered runs first).
pub fn at_exit(hook: impl FnOnce() + Send + 'static) {
    HOOKS
        .lock()
        .unwrap_or_else(|e| e.into_inner())
        .push(Box::new(hook));
}

/// Run (once) the registered hooks.
pub fn run_hooks() {
    let hooks = std::mem::take(&mut *HOOKS.lock().unwrap_or_else(|e| e.into_inner()));
    for hook in hooks.into_iter().rev() {
        hook();
    }
}

/// `std::process::exit` after running the exit hooks.
pub fn exit(code: i32) -> ! {
    run_hooks();
    std::process::exit(code)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[test]
    fn hooks_run_once_in_reverse_order() {
        static ORDER: Mutex<Vec<u8>> = Mutex::new(Vec::new());
        static RUNS: AtomicUsize = AtomicUsize::new(0);
        at_exit(|| ORDER.lock().unwrap().push(1));
        at_exit(|| {
            ORDER.lock().unwrap().push(2);
            RUNS.fetch_add(1, Ordering::SeqCst);
        });
        run_hooks();
        run_hooks();
        assert_eq!(*ORDER.lock().unwrap(), vec![2, 1]);
        assert_eq!(RUNS.load(Ordering::SeqCst), 1);
    }
}
