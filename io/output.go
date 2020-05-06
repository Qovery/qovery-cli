package io

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"os"
)

func PrintError(content string) {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("%s:   %s\n", red("ERROR"), content)
}

func PrintSolution(content string) {
	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s: %s\n", green("SOLUTION"), content)
}

func PrintHint(content string) {
	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s: %s\n", green("HINT"), content)
}

func GetTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("|")
	return table
}
