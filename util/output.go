package util

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

func printSolution(content string) {
	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s: %s\n", green("SOLUTION"), content)
}

func GetTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("|")
	/*table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("|")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)*/
	return table
}
