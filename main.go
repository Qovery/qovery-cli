package main

import (
	"fmt"
	"os"
	"qovery-cli/cmd"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
