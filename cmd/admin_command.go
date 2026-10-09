package cmd

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

func runInteractiveCommand(commandPath string, args, environment []string) error {
	return newInteractiveCommand(commandPath, args, environment).Run()
}

func runInteractiveCommandWithSignalForwarding(commandPath string, args, environment []string) error {
	command := newInteractiveCommand(commandPath, args, environment)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, interactiveCommandSignals()...)
	defer signal.Stop(signals)

	return runCommandWithSignalForwarding(command, signals)
}

func newInteractiveCommand(commandPath string, args, environment []string) *exec.Cmd {
	command := exec.Command(commandPath, args...)
	command.Stdout = os.Stdout
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	if environment != nil {
		command.Env = environment
	}
	return command
}

func runCommandWithSignalForwarding(command *exec.Cmd, signals <-chan os.Signal) error {
	if err := command.Start(); err != nil {
		return err
	}

	finished := make(chan struct{})
	forwarderDone := make(chan struct{})
	go func() {
		defer close(forwarderDone)
		for {
			select {
			case received := <-signals:
				if received == nil {
					return
				}
				if err := command.Process.Signal(received); err != nil && !errors.Is(err, os.ErrProcessDone) {
					_ = command.Process.Kill()
				}
			case <-finished:
				return
			}
		}
	}()

	err := command.Wait()
	close(finished)
	<-forwarderDone
	return err
}

func environmentWithKubeconfig(kubeconfigPath string) []string {
	environment := os.Environ()
	result := make([]string, 0, len(environment)+1)
	replaced := false
	for _, entry := range environment {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(key, "KUBECONFIG") {
			if replaced {
				continue
			}
			entry = "KUBECONFIG=" + kubeconfigPath
			replaced = true
		}
		result = append(result, entry)
	}
	if !replaced {
		result = append(result, "KUBECONFIG="+kubeconfigPath)
	}
	return result
}
