package skills

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/qovery/qovery-cli/utils"
)

const (
	// DisablePromptEnv turns off the install offer made after 'qovery auth'
	DisablePromptEnv = "QOVERY_NO_SKILLS_PROMPT"

	offerEventName = "cli-skills-install-offer"
	stateFileName  = "skills-prompt.json"

	// After this many refusals the offer goes quiet for snoozeDuration, then is made once more
	maxDeclinesBeforeSnooze = 3
	snoozeDuration          = 90 * 24 * time.Hour
)

// Offer outcomes, sent as the "action" property of the telemetry event
const (
	actionHint          = "hint"
	actionAccepted      = "accepted"
	actionInstallFailed = "install_failed"
	actionDeclined      = "declined"
	actionNever         = "never"
)

type promptState struct {
	Declines       int       `json:"declines"`
	LastDeclinedAt time.Time `json:"last_declined_at,omitempty"`
	Never          bool      `json:"never,omitempty"`
}

func (s promptState) shouldAsk(now time.Time) bool {
	if s.Never {
		return false
	}
	if s.Declines < maxDeclinesBeforeSnooze {
		return true
	}
	return now.Sub(s.LastDeclinedAt) >= snoozeDuration
}

type offer struct {
	in          io.Reader
	out         io.Writer
	errOut      io.Writer
	home        string
	cwd         string
	statePath   string
	interactive bool
	headless    bool
	getenv      func(string) string
	now         func() time.Time
	install     func(stdout io.Writer, stderr io.Writer) error
	track       func(action string, properties posthog.Properties)
}

// OfferAfterAuth suggests installing the Qovery skills right after a successful 'qovery auth'.
// It never blocks a script or an agent: in headless mode or without a terminal it only prints a one-line tip on stderr.
func OfferAfterAuth(cmd *cobra.Command, headless bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = home
	}
	qoveryDir, err := utils.QoveryDirPath()
	if err != nil {
		return
	}

	offer{
		in:          os.Stdin,
		out:         os.Stdout,
		errOut:      os.Stderr,
		home:        home,
		cwd:         cwd,
		statePath:   filepath.Join(qoveryDir, stateFileName),
		interactive: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		headless:    headless,
		getenv:      os.Getenv,
		now:         time.Now,
		install: func(stdout io.Writer, stderr io.Writer) error {
			return Install(false, stdout, stderr)
		},
		track: func(action string, properties posthog.Properties) {
			utils.CaptureWithEventAndProperties(cmd, offerEventName, properties.Set("action", action))
		},
	}.run()
}

func (o offer) run() {
	if o.getenv(DisablePromptEnv) != "" || o.getenv("CI") != "" {
		return
	}
	if len(findInstallations(o.home, o.cwd)) > 0 {
		return
	}

	agentTools := detectAgentTools(o.home)
	properties := posthog.Properties{"agent_tools": strings.Join(agentTools, ",")}

	// An agent or a script ran 'qovery auth': a question would hang it, a tip on stderr will not.
	// Headless auth gets the tip too, as agents running it may be attached to a pseudo-terminal
	if o.headless || !o.interactive {
		_, _ = fmt.Fprintln(o.errOut, "Tip: install the Qovery skills so AI coding agents can deploy, troubleshoot and optimize apps on Qovery: qovery skills install")
		o.track(actionHint, properties)
		return
	}

	// Without an AI coding tool on this machine, the skills would not be used
	if len(agentTools) == 0 {
		return
	}

	state := o.loadState()
	now := o.now()
	if !state.shouldAsk(now) {
		return
	}
	properties.Set("previous_declines", state.Declines)

	o.printPitch(agentTools)
	switch o.readAnswer() {
	case actionAccepted:
		_, _ = fmt.Fprintln(o.out)
		if err := o.install(o.out, o.errOut); err != nil {
			utils.PrintlnError(err)
			_, _ = fmt.Fprintf(o.out, "You can install them later with 'qovery skills install' or '%s'\n", ManualInstallCommand)
			o.track(actionInstallFailed, properties)
			return
		}
		o.track(actionAccepted, properties)
	case actionNever:
		state.Never = true
		if err := o.saveState(state); err != nil {
			_, _ = fmt.Fprintf(o.out, "Could not save your choice (%s). Set %s=1 to turn this offer off.\n", err, DisablePromptEnv)
		} else {
			_, _ = fmt.Fprintln(o.out, "Got it, you won't be asked again. Install them anytime with 'qovery skills install'.")
		}
		o.track(actionNever, properties)
	default:
		state.Declines++
		state.LastDeclinedAt = now
		_ = o.saveState(state)
		_, _ = fmt.Fprintln(o.out, "No problem. Install them anytime with 'qovery skills install'.")
		o.track(actionDeclined, properties)
	}
}

func (o offer) printPitch(agentTools []string) {
	_, _ = fmt.Fprintf(o.out, `
Your AI coding agent doesn't know Qovery yet (found: %s).
The Qovery skills let it:
  • deploy any repository to your cluster from one prompt
  • find the cause of a failed deployment from its logs
  • spot wasted CPU, memory and idle environments
  • create a preview environment for each pull request
It takes a few seconds and only writes to ~/.claude/skills, ~/.config/opencode/skills and ~/.agents/skills.

`, strings.Join(agentTools, ", "))
	_, _ = fmt.Fprint(o.out, "Install the Qovery skills now? [Y/n/never] ")
}

func (o offer) readAnswer() string {
	line, err := bufio.NewReader(o.in).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	switch {
	case answer == "" && err != nil:
		// stdin closed before any answer: do not install anything without consent
		return actionDeclined
	case answer == "" || answer == "y" || answer == "yes":
		return actionAccepted
	case answer == "never":
		return actionNever
	default:
		return actionDeclined
	}
}

func (o offer) loadState() promptState {
	state := promptState{}
	content, err := os.ReadFile(o.statePath)
	if err != nil {
		return state
	}
	_ = json.Unmarshal(content, &state)
	return state
}

func (o offer) saveState(state promptState) error {
	content, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(o.statePath), 0700); err != nil {
		return err
	}
	return os.WriteFile(o.statePath, content, 0600)
}
