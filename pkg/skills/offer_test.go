package skills

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
)

type offerFixture struct {
	offer     offer
	out       *bytes.Buffer
	errOut    *bytes.Buffer
	installs  int
	tracked   []string
	env       map[string]string
	installFn func() error
}

func newOfferFixture(t *testing.T, answer string) *offerFixture {
	home := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0700))

	f := &offerFixture{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}, env: map[string]string{}}
	f.offer = offer{
		in:          strings.NewReader(answer),
		out:         f.out,
		errOut:      f.errOut,
		home:        home,
		cwd:         t.TempDir(),
		statePath:   filepath.Join(home, ".qovery", stateFileName),
		interactive: true,
		getenv:      func(key string) string { return f.env[key] },
		now:         func() time.Time { return time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC) },
		install: func(stdout io.Writer, stderr io.Writer) error {
			f.installs++
			if f.installFn != nil {
				return f.installFn()
			}
			return nil
		},
		track: func(action string, properties posthog.Properties) { f.tracked = append(f.tracked, action) },
	}
	return f
}

func installSkillAt(t *testing.T, base string, version string) {
	dir := filepath.Join(base, routerSkill)
	assert.NoError(t, os.MkdirAll(dir, 0700))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: qovery\n---\n"), 0600))
	if version != "" {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, versionFile), []byte(version+"\n"), 0600))
	}
}

func TestOfferInstallsOnEnterOrYes(t *testing.T) {
	for _, answer := range []string{"\n", "y\n", "YES\n"} {
		f := newOfferFixture(t, answer)
		f.offer.run()

		assert.Equal(t, 1, f.installs, "answer %q", answer)
		assert.Equal(t, []string{actionAccepted}, f.tracked)
		assert.Contains(t, f.out.String(), "found: Claude Code")
	}
}

func TestOfferDeclineIsCountedAndAskedAgain(t *testing.T) {
	f := newOfferFixture(t, "n\n")
	f.offer.run()
	f.offer.in = strings.NewReader("no\n")
	f.offer.run()

	assert.Equal(t, 0, f.installs)
	assert.Equal(t, []string{actionDeclined, actionDeclined}, f.tracked)
	assert.Equal(t, 2, f.offer.loadState().Declines)
}

func TestOfferGoesQuietAfterThreeDeclinesThenAsksAgain(t *testing.T) {
	f := newOfferFixture(t, "")
	now := f.offer.now()
	assert.NoError(t, f.offer.saveState(promptState{Declines: 3, LastDeclinedAt: now.Add(-snoozeDuration + time.Hour)}))

	f.offer.in = strings.NewReader("n\n")
	f.offer.run()
	assert.Empty(t, f.out.String(), "still within the quiet period")
	assert.Empty(t, f.tracked)

	f.offer.now = func() time.Time { return now.Add(time.Hour) }
	f.offer.run()
	assert.Contains(t, f.out.String(), "[Y/n/never]")
	assert.Equal(t, 4, f.offer.loadState().Declines)
}

func TestOfferNeverStopsAsking(t *testing.T) {
	f := newOfferFixture(t, "never\n")
	f.offer.run()
	assert.True(t, f.offer.loadState().Never)

	f.out.Reset()
	f.offer.in = strings.NewReader("y\n")
	f.offer.run()
	assert.Empty(t, f.out.String())
	assert.Equal(t, 0, f.installs)
}

func TestOfferClosedStdinDoesNotInstall(t *testing.T) {
	f := newOfferFixture(t, "")
	f.offer.run()
	assert.Equal(t, 0, f.installs)
	assert.Equal(t, []string{actionDeclined}, f.tracked)
}

func TestOfferReportsInstallFailure(t *testing.T) {
	f := newOfferFixture(t, "y\n")
	f.installFn = func() error { return errors.New("network down") }
	f.offer.run()
	assert.Equal(t, []string{actionInstallFailed}, f.tracked)
	assert.Contains(t, f.out.String(), "qovery skills install")
}

func TestOfferNonInteractivePrintsTipOnly(t *testing.T) {
	f := newOfferFixture(t, "y\n")
	f.offer.interactive = false
	f.offer.run()

	assert.Equal(t, 0, f.installs)
	assert.Empty(t, f.out.String())
	assert.Contains(t, f.errOut.String(), "qovery skills install")
	assert.Equal(t, []string{actionHint}, f.tracked)
}

func TestOfferHeadlessPrintsTipEvenInTerminal(t *testing.T) {
	f := newOfferFixture(t, "y\n")
	f.offer.headless = true
	f.offer.run()

	assert.Equal(t, 0, f.installs)
	assert.Empty(t, f.out.String())
	assert.Contains(t, f.errOut.String(), "qovery skills install")
	assert.Equal(t, []string{actionHint}, f.tracked)
}

func TestOfferNeverReportsUnsavedChoice(t *testing.T) {
	f := newOfferFixture(t, "never\n")
	// A file where the state directory should be makes the save fail
	assert.NoError(t, os.WriteFile(filepath.Dir(f.offer.statePath), nil, 0600))
	f.offer.run()

	assert.Contains(t, f.out.String(), DisablePromptEnv)
	assert.NotContains(t, f.out.String(), "won't be asked again")
}

func TestOfferSkipped(t *testing.T) {
	t.Run("skills already installed", func(t *testing.T) {
		f := newOfferFixture(t, "y\n")
		installSkillAt(t, filepath.Join(f.offer.home, ".agents", "skills"), "abc1234")
		f.offer.run()
		assert.Empty(t, f.out.String()+f.errOut.String())
	})
	t.Run("installed in the project", func(t *testing.T) {
		f := newOfferFixture(t, "y\n")
		installSkillAt(t, filepath.Join(f.offer.cwd, ".claude", "skills"), "")
		f.offer.run()
		assert.Empty(t, f.out.String()+f.errOut.String())
	})
	t.Run("opt-out variable", func(t *testing.T) {
		f := newOfferFixture(t, "y\n")
		f.env[DisablePromptEnv] = "1"
		f.offer.run()
		assert.Empty(t, f.out.String()+f.errOut.String())
	})
	t.Run("CI", func(t *testing.T) {
		f := newOfferFixture(t, "y\n")
		f.env["CI"] = "true"
		f.offer.interactive = false
		f.offer.run()
		assert.Empty(t, f.out.String()+f.errOut.String())
	})
	t.Run("no AI coding tool", func(t *testing.T) {
		f := newOfferFixture(t, "y\n")
		assert.NoError(t, os.RemoveAll(filepath.Join(f.offer.home, ".claude")))
		f.offer.run()
		assert.Empty(t, f.out.String())
		assert.Empty(t, f.tracked)
	})
}

func TestFindInstallationsReadsVersionAndDeduplicates(t *testing.T) {
	home := t.TempDir()
	installSkillAt(t, filepath.Join(home, ".claude", "skills"), "abc1234")
	installSkillAt(t, filepath.Join(home, ".agents", "skills"), "")

	// From the home directory, project locations are the global ones: each must be listed once
	installations := findInstallations(home, home)
	assert.Equal(t, []Installation{
		{Path: filepath.Join(home, ".claude", "skills", "qovery"), Version: "abc1234"},
		{Path: filepath.Join(home, ".agents", "skills", "qovery"), Version: "unknown"},
	}, installations)
}

func TestInstallerArgs(t *testing.T) {
	assert.Equal(t, []string{"--global"}, installerArgs(false, false))
	assert.Equal(t, []string{"--project"}, installerArgs(true, false))
	assert.Equal(t, []string{"--uninstall"}, installerArgs(false, true))
}
