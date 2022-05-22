package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var parseHerokuJson bool

var envParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse environment variables and create .env (dot env) file",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) > 1 {
			utils.PrintlnError(fmt.Errorf("more than one arg is not allowed"))
			return
		}

		if !parseHerokuJson {
			utils.PrintlnError(fmt.Errorf("you need to specify an option like '--heroku-json' or other. Type -h to see different options"))
			return
		}

		dotEnvFilePath := ""
		if len(args) == 0 && !parseHerokuJson {
			file, err := scanAndSelectDotEnvFile()
			if err != nil {
				utils.PrintlnError(err)
				return
			}

			dotEnvFilePath = file
		} else if len(args) == 1 && !parseHerokuJson {
			dotEnvFilePath = args[0]
		} else if !parseHerokuJson {
			utils.PrintlnError(fmt.Errorf("more than one arg is not allowed"))
			return
		}

		if dotEnvFilePath == "" && !parseHerokuJson {
			utils.PrintlnError(fmt.Errorf("no dot env file specified"))
			return
		}

		var envs map[string]string
		var err error

		if parseHerokuJson {
			envs = make(map[string]string)
			err = json.NewDecoder(os.Stdin).Decode(&envs)
		}

		if err != nil {
			utils.PrintlnError(err)
			utils.Println("Did you execute 'heroku config -a <your_heroku_app_name> --json' command?")
			return
		}

		for key, value := range envs {
			fmt.Println(fmt.Sprintf("%s=%s", key, value))
		}
	},
}

func init() {
	envCmd.AddCommand(envParseCmd)
	envParseCmd.Flags().BoolVarP(&parseHerokuJson, "heroku-json", "j", false, "Parse environment variables from a Heroku JSON payload")
}
