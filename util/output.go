package util

import (
	"fmt"
	"github.com/fatih/color"
)

func PrintError(content string) {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("%s: %s\n", red("ERROR"), content)
}

func printSolution(content string) {
	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s: %s\n", green("SOLUTION"), content)
}