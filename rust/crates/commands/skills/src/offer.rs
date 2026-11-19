//! Port of `pkg/skills/offer.go`: the skills install offer made after `qovery auth`.

use crate::installer::{
    MANUAL_INSTALL_COMMAND, detect_agent_tools, find_installations_in, user_home_dir,
};
use chrono::{DateTime, Duration, Utc};
use serde_json::{Map, Value, json};
use std::io::{BufRead, Read, Write};
use std::path::PathBuf;

/// `DisablePromptEnv`: turns off the install offer made after 'qovery auth'.
pub const DISABLE_PROMPT_ENV: &str = "QOVERY_NO_SKILLS_PROMPT";

const OFFER_EVENT_NAME: &str = "cli-skills-install-offer";
const STATE_FILE_NAME: &str = "skills-prompt.json";

/// After this many refusals the offer goes quiet for `snooze_duration`, then is made once more.
const MAX_DECLINES_BEFORE_SNOOZE: i64 = 3;
fn snooze_duration() -> Duration {
    Duration::hours(90 * 24)
}

const ACTION_HINT: &str = "hint";
const ACTION_ACCEPTED: &str = "accepted";
const ACTION_INSTALL_FAILED: &str = "install_failed";
const ACTION_DECLINED: &str = "declined";
const ACTION_NEVER: &str = "never";

/// Go zero `time.Time`.
fn zero_time() -> DateTime<Utc> {
    DateTime::parse_from_rfc3339("0001-01-01T00:00:00Z")
        .unwrap()
        .with_timezone(&Utc)
}

#[derive(Debug, Clone, PartialEq)]
struct PromptState {
    declines: i64,
    last_declined_at: DateTime<Utc>,
    never: bool,
}

impl Default for PromptState {
    fn default() -> Self {
        PromptState {
            declines: 0,
            last_declined_at: zero_time(),
            never: false,
        }
    }
}

impl PromptState {
    fn should_ask(&self, now: DateTime<Utc>) -> bool {
        if self.never {
            return false;
        }
        if self.declines < MAX_DECLINES_BEFORE_SNOOZE {
            return true;
        }
        now - self.last_declined_at >= snooze_duration()
    }

    /// Go `json.Unmarshal` into `promptState` (fields of the wrong type are skipped).
    fn from_json(content: &[u8]) -> PromptState {
        let mut state = PromptState::default();
        let Ok(Value::Object(m)) = serde_json::from_slice::<Value>(content) else {
            return state;
        };
        for (k, v) in m {
            match k.to_lowercase().as_str() {
                "declines" => {
                    if let Some(n) = v.as_i64() {
                        state.declines = n;
                    }
                }
                "last_declined_at" => {
                    if let Some(t) = v
                        .as_str()
                        .and_then(|s| DateTime::parse_from_rfc3339(s).ok())
                    {
                        state.last_declined_at = t.with_timezone(&Utc);
                    }
                }
                "never" => {
                    if let Some(b) = v.as_bool() {
                        state.never = b;
                    }
                }
                _ => {}
            }
        }
        state
    }

    /// Go `json.Marshal(promptState)` (`omitempty` does not apply to a `time.Time` struct).
    fn to_json(&self) -> String {
        let t = self.last_declined_at.with_timezone(&chrono::Local);
        let mut s = format!(
            "{{\"declines\":{},\"last_declined_at\":\"{}\"",
            self.declines,
            if self.last_declined_at == zero_time() {
                "0001-01-01T00:00:00Z".to_string()
            } else {
                qovery_core::auth::go_time_json(t)
            }
        );
        if self.never {
            s.push_str(",\"never\":true");
        }
        s.push('}');
        s
    }
}

type TrackFn<'a> = Box<dyn FnMut(&str, Map<String, Value>) + 'a>;
type InstallFn<'a> = Box<dyn FnMut() -> Result<(), String> + 'a>;

struct Offer<'a> {
    input: Box<dyn Read + 'a>,
    out: Box<dyn Write + 'a>,
    err_out: Box<dyn Write + 'a>,
    home: PathBuf,
    cwd: PathBuf,
    state_path: PathBuf,
    interactive: bool,
    headless: bool,
    getenv: Box<dyn Fn(&str) -> String + 'a>,
    now: Box<dyn Fn() -> DateTime<Utc> + 'a>,
    install: InstallFn<'a>,
    track: TrackFn<'a>,
}

/// `OfferAfterAuth`: suggest installing the Qovery skills right after a successful `qovery auth`.
/// Never blocks a script or an agent: headless or without a terminal it only prints a one-line tip on stderr.
pub fn offer_after_auth(ctx: &qovery_cobra::Ctx, headless: bool) {
    use is_terminal::IsTerminal;
    let Some(home) = user_home_dir() else { return };
    let cwd = std::env::current_dir().unwrap_or_else(|_| home.clone());
    let Ok(qovery_dir) = qovery_core::context::qovery_dir_path() else {
        return;
    };
    let info = qovery_core::telemetry::CommandInfo::from_ctx(ctx);

    Offer {
        input: Box::new(std::io::stdin()),
        out: Box::new(std::io::stdout()),
        err_out: Box::new(std::io::stderr()),
        home,
        cwd,
        state_path: qovery_dir.join(STATE_FILE_NAME),
        interactive: std::io::stdin().is_terminal() && std::io::stdout().is_terminal(),
        headless,
        getenv: Box::new(|k| std::env::var(k).unwrap_or_default()),
        now: Box::new(Utc::now),
        install: Box::new(|| crate::installer::install(false)),
        track: Box::new(move |action, mut properties| {
            properties.insert("action".into(), json!(action));
            qovery_core::telemetry::capture_with_event_and_properties(
                &info,
                OFFER_EVENT_NAME,
                properties,
            );
        }),
    }
    .run();
}

impl Offer<'_> {
    fn run(&mut self) {
        if !(self.getenv)(DISABLE_PROMPT_ENV).is_empty() || !(self.getenv)("CI").is_empty() {
            return;
        }
        if !find_installations_in(&self.home, &self.cwd).is_empty() {
            return;
        }

        let agent_tools = detect_agent_tools(&self.home);
        let mut properties = Map::new();
        properties.insert("agent_tools".into(), json!(agent_tools.join(",")));

        // An agent or a script ran 'qovery auth': a question would hang it, a tip on stderr will not
        if self.headless || !self.interactive {
            let _ = writeln!(
                self.err_out,
                "Tip: install the Qovery skills so AI coding agents can deploy, troubleshoot and optimize apps on Qovery: qovery skills install"
            );
            let _ = self.err_out.flush();
            (self.track)(ACTION_HINT, properties);
            return;
        }

        // Without an AI coding tool on this machine, the skills would not be used
        if agent_tools.is_empty() {
            return;
        }

        let mut state = self.load_state();
        let now = (self.now)();
        if !state.should_ask(now) {
            return;
        }
        properties.insert("previous_declines".into(), json!(state.declines));

        self.print_pitch(&agent_tools);
        match self.read_answer() {
            ACTION_ACCEPTED => {
                let _ = writeln!(self.out);
                let _ = self.out.flush();
                if let Err(e) = (self.install)() {
                    qovery_log::println_error(e);
                    let _ = writeln!(
                        self.out,
                        "You can install them later with 'qovery skills install' or '{MANUAL_INSTALL_COMMAND}'"
                    );
                    let _ = self.out.flush();
                    (self.track)(ACTION_INSTALL_FAILED, properties);
                    return;
                }
                (self.track)(ACTION_ACCEPTED, properties);
            }
            ACTION_NEVER => {
                state.never = true;
                match self.save_state(&state) {
                    Err(e) => {
                        let _ = writeln!(
                            self.out,
                            "Could not save your choice ({e}). Set {DISABLE_PROMPT_ENV}=1 to turn this offer off."
                        );
                    }
                    Ok(()) => {
                        let _ = writeln!(
                            self.out,
                            "Got it, you won't be asked again. Install them anytime with 'qovery skills install'."
                        );
                    }
                }
                let _ = self.out.flush();
                (self.track)(ACTION_NEVER, properties);
            }
            _ => {
                state.declines += 1;
                state.last_declined_at = now;
                let _ = self.save_state(&state);
                let _ = writeln!(
                    self.out,
                    "No problem. Install them anytime with 'qovery skills install'."
                );
                let _ = self.out.flush();
                (self.track)(ACTION_DECLINED, properties);
            }
        }
    }

    fn print_pitch(&mut self, agent_tools: &[String]) {
        let _ = write!(
            self.out,
            "
Your AI coding agent doesn't know Qovery yet (found: {}).
The Qovery skills let it:
  • deploy any repository to your cluster from one prompt
  • find the cause of a failed deployment from its logs
  • spot wasted CPU, memory and idle environments
  • create a preview environment for each pull request
It takes a few seconds and only writes to ~/.claude/skills, ~/.config/opencode/skills and ~/.agents/skills.

",
            agent_tools.join(", ")
        );
        let _ = write!(self.out, "Install the Qovery skills now? [Y/n/never] ");
        let _ = self.out.flush();
    }

    fn read_answer(&mut self) -> &'static str {
        let mut line = Vec::new();
        let res = std::io::BufReader::new(&mut self.input).read_until(b'\n', &mut line);
        // Go ReadString returns an error (io.EOF) when the line does not end with the delimiter
        let eof = res.is_err() || !line.ends_with(b"\n");
        let answer = String::from_utf8_lossy(&line).trim().to_lowercase();
        if answer.is_empty() && eof {
            // stdin closed before any answer: do not install anything without consent
            return ACTION_DECLINED;
        }
        match answer.as_str() {
            "" | "y" | "yes" => ACTION_ACCEPTED,
            "never" => ACTION_NEVER,
            _ => ACTION_DECLINED,
        }
    }

    fn load_state(&self) -> PromptState {
        match std::fs::read(&self.state_path) {
            Ok(content) => PromptState::from_json(&content),
            Err(_) => PromptState::default(),
        }
    }

    fn save_state(&self, state: &PromptState) -> Result<(), String> {
        let content = state.to_json();
        let dir = self
            .state_path
            .parent()
            .map(|p| p.to_path_buf())
            .unwrap_or_default();
        create_dir_all_0700(&dir)
            .map_err(|e| qovery_core::context::go_path_error("mkdir", &dir, &e))?;
        write_0600(&self.state_path, content.as_bytes())
            .map_err(|e| qovery_core::context::go_path_error("open", &self.state_path, &e))
    }
}

fn create_dir_all_0700(dir: &std::path::Path) -> std::io::Result<()> {
    let mut b = std::fs::DirBuilder::new();
    b.recursive(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::DirBuilderExt;
        b.mode(0o700);
    }
    match b.create(dir) {
        Ok(()) => Ok(()),
        Err(e) => {
            // Go MkdirAll: "not a directory" when a file is in the way
            if dir.exists() && !dir.is_dir() {
                Err(std::io::Error::other("not a directory"))
            } else {
                Err(e)
            }
        }
    }
}

fn write_0600(path: &std::path::Path, content: &[u8]) -> std::io::Result<()> {
    let mut opts = std::fs::OpenOptions::new();
    opts.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(0o600);
    }
    opts.open(path)?.write_all(content)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::installer::tests::install_skill_at;
    use std::cell::RefCell;
    use std::collections::HashMap;
    use std::rc::Rc;

    #[derive(Clone, Default)]
    struct Shared {
        out: Rc<RefCell<Vec<u8>>>,
        err_out: Rc<RefCell<Vec<u8>>>,
        installs: Rc<RefCell<usize>>,
        tracked: Rc<RefCell<Vec<String>>>,
        env: Rc<RefCell<HashMap<String, String>>>,
        install_fails: Rc<RefCell<bool>>,
        now: Rc<RefCell<DateTime<Utc>>>,
    }

    struct SharedWriter(Rc<RefCell<Vec<u8>>>);
    impl Write for SharedWriter {
        fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
            self.0.borrow_mut().extend_from_slice(buf);
            Ok(buf.len())
        }
        fn flush(&mut self) -> std::io::Result<()> {
            Ok(())
        }
    }

    struct Fixture {
        _home: tempfile::TempDir,
        _cwd: tempfile::TempDir,
        home: PathBuf,
        cwd: PathBuf,
        shared: Shared,
        interactive: bool,
        headless: bool,
    }

    impl Fixture {
        fn new() -> Fixture {
            let home = tempfile::tempdir().unwrap();
            let cwd = tempfile::tempdir().unwrap();
            std::fs::create_dir_all(home.path().join(".claude")).unwrap();
            let shared = Shared::default();
            *shared.now.borrow_mut() = DateTime::parse_from_rfc3339("2026-11-18T10:00:00Z")
                .unwrap()
                .with_timezone(&Utc);
            Fixture {
                home: home.path().to_path_buf(),
                cwd: cwd.path().to_path_buf(),
                _home: home,
                _cwd: cwd,
                shared,
                interactive: true,
                headless: false,
            }
        }

        fn state_path(&self) -> PathBuf {
            self.home.join(".qovery").join(STATE_FILE_NAME)
        }

        fn offer(&self, answer: &str) -> Offer<'static> {
            let s = self.shared.clone();
            let (s1, s2, s3, s4, s5) = (s.clone(), s.clone(), s.clone(), s.clone(), s.clone());
            Offer {
                input: Box::new(std::io::Cursor::new(answer.as_bytes().to_vec())),
                out: Box::new(SharedWriter(s.out.clone())),
                err_out: Box::new(SharedWriter(s.err_out.clone())),
                home: self.home.clone(),
                cwd: self.cwd.clone(),
                state_path: self.state_path(),
                interactive: self.interactive,
                headless: self.headless,
                getenv: Box::new(move |k| s1.env.borrow().get(k).cloned().unwrap_or_default()),
                now: Box::new(move || *s2.now.borrow()),
                install: Box::new(move || {
                    *s3.installs.borrow_mut() += 1;
                    if *s4.install_fails.borrow() {
                        Err("network down".to_string())
                    } else {
                        Ok(())
                    }
                }),
                track: Box::new(move |action, _| s5.tracked.borrow_mut().push(action.to_string())),
            }
        }

        fn run(&self, answer: &str) {
            self.offer(answer).run();
        }
        fn out(&self) -> String {
            String::from_utf8_lossy(&self.shared.out.borrow()).to_string()
        }
        fn err_out(&self) -> String {
            String::from_utf8_lossy(&self.shared.err_out.borrow()).to_string()
        }
        fn installs(&self) -> usize {
            *self.shared.installs.borrow()
        }
        fn tracked(&self) -> Vec<String> {
            self.shared.tracked.borrow().clone()
        }
        fn state(&self) -> PromptState {
            self.offer("").load_state()
        }
    }

    #[test]
    fn installs_on_enter_or_yes() {
        for answer in ["\n", "y\n", "YES\n"] {
            let f = Fixture::new();
            f.run(answer);
            assert_eq!(f.installs(), 1, "answer {answer:?}");
            assert_eq!(f.tracked(), vec![ACTION_ACCEPTED]);
            assert!(f.out().contains("found: Claude Code"));
        }
    }

    #[test]
    fn decline_is_counted_and_asked_again() {
        let f = Fixture::new();
        f.run("n\n");
        f.run("no\n");
        assert_eq!(f.installs(), 0);
        assert_eq!(f.tracked(), vec![ACTION_DECLINED, ACTION_DECLINED]);
        assert_eq!(f.state().declines, 2);
    }

    #[test]
    fn goes_quiet_after_three_declines_then_asks_again() {
        let f = Fixture::new();
        let now = *f.shared.now.borrow();
        f.offer("")
            .save_state(&PromptState {
                declines: 3,
                last_declined_at: now - snooze_duration() + Duration::hours(1),
                never: false,
            })
            .unwrap();
        f.run("n\n");
        assert!(f.out().is_empty(), "still within the quiet period");
        assert!(f.tracked().is_empty());

        *f.shared.now.borrow_mut() = now + Duration::hours(1);
        f.run("n\n");
        assert!(f.out().contains("[Y/n/never]"));
        assert_eq!(f.state().declines, 4);
    }

    #[test]
    fn never_stops_asking() {
        let f = Fixture::new();
        f.run("never\n");
        assert!(f.state().never);
        f.shared.out.borrow_mut().clear();
        f.run("y\n");
        assert!(f.out().is_empty());
        assert_eq!(f.installs(), 0);
    }

    #[test]
    fn closed_stdin_does_not_install() {
        let f = Fixture::new();
        f.run("");
        assert_eq!(f.installs(), 0);
        assert_eq!(f.tracked(), vec![ACTION_DECLINED]);
    }

    #[test]
    fn reports_install_failure() {
        let f = Fixture::new();
        *f.shared.install_fails.borrow_mut() = true;
        f.run("y\n");
        assert_eq!(f.tracked(), vec![ACTION_INSTALL_FAILED]);
        assert!(f.out().contains("qovery skills install"));
    }

    #[test]
    fn non_interactive_prints_tip_only() {
        let mut f = Fixture::new();
        f.interactive = false;
        f.run("y\n");
        assert_eq!(f.installs(), 0);
        assert!(f.out().is_empty());
        assert!(f.err_out().contains("qovery skills install"));
        assert_eq!(f.tracked(), vec![ACTION_HINT]);
    }

    #[test]
    fn headless_prints_tip_even_in_terminal() {
        let mut f = Fixture::new();
        f.headless = true;
        f.run("y\n");
        assert_eq!(f.installs(), 0);
        assert!(f.out().is_empty());
        assert!(f.err_out().contains("qovery skills install"));
        assert_eq!(f.tracked(), vec![ACTION_HINT]);
    }

    #[test]
    fn never_reports_unsaved_choice() {
        let f = Fixture::new();
        // A file where the state directory should be makes the save fail
        std::fs::write(f.state_path().parent().unwrap(), b"").unwrap();
        f.run("never\n");
        assert!(f.out().contains(DISABLE_PROMPT_ENV));
        assert!(!f.out().contains("won't be asked again"));
    }

    #[test]
    fn skipped() {
        let f = Fixture::new();
        install_skill_at(&f.home.join(".agents").join("skills"), "abc1234");
        f.run("y\n");
        assert!((f.out() + &f.err_out()).is_empty());

        let f = Fixture::new();
        install_skill_at(&f.cwd.join(".claude").join("skills"), "");
        f.run("y\n");
        assert!((f.out() + &f.err_out()).is_empty());

        let f = Fixture::new();
        f.shared
            .env
            .borrow_mut()
            .insert(DISABLE_PROMPT_ENV.into(), "1".into());
        f.run("y\n");
        assert!((f.out() + &f.err_out()).is_empty());

        let mut f = Fixture::new();
        f.shared.env.borrow_mut().insert("CI".into(), "true".into());
        f.interactive = false;
        f.run("y\n");
        assert!((f.out() + &f.err_out()).is_empty());

        let f = Fixture::new();
        std::fs::remove_dir_all(f.home.join(".claude")).unwrap();
        f.run("y\n");
        assert!(f.out().is_empty());
        assert!(f.tracked().is_empty());
    }

    #[test]
    fn state_json() {
        let s = PromptState::default();
        assert_eq!(
            s.to_json(),
            "{\"declines\":0,\"last_declined_at\":\"0001-01-01T00:00:00Z\"}"
        );
        let s = PromptState {
            never: true,
            ..Default::default()
        };
        assert!(s.to_json().ends_with(",\"never\":true}"));
        assert_eq!(PromptState::from_json(s.to_json().as_bytes()), s);
    }
}
