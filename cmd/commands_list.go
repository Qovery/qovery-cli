package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var listCmd = &cobra.Command{
	Use:   "list-commands",
	Short: "List all available commands with descriptions, aliases, args, and flags",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available commands:")
		printCommandsRecursive(rootCmd, "")
	},
}

func printCommandsRecursive(cmd *cobra.Command, parentPath string) {
	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}

		fullCmd := strings.TrimSpace(parentPath + " " + c.Name())
		aliases := ""
		if len(c.Aliases) > 0 {
			aliases = fmt.Sprintf(" (aliases: %s)", strings.Join(c.Aliases, ", "))
		}

		fmt.Printf("  %s: %s%s\n", fullCmd, c.Short, aliases)

		if c.Use != c.Name() {
			fmt.Printf("    Usage: %s\n", c.UseLine())
		}

		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			defVal := f.DefValue
			if f.Value.Type() == "string" && defVal == "" {
				defVal = `""`
			}

			short := ""
			if f.Shorthand != "" {
				short = fmt.Sprintf("-%s, ", f.Shorthand)
			}

			fmt.Printf("    Flag: %s--%s (%s), default: %s\n", short, f.Name, f.Value.Type(), defVal)
		})

		printCommandsRecursive(c, fullCmd)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
