package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"sort"
	"strconv"
	"strings"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Do project initialization to use Qovery",
	Long: `INIT do project initialization to use Qovery within the current directory. For example:

	qovery init`,
	Run: func(cmd *cobra.Command, args []string) {

		if _, err := os.Stat(".qovery.yml"); err == nil {
			fmt.Println("You already have a .qovery.yml file")
			os.Exit(0)
		}

		if util.CurrentBranchName() == "" {
			fmt.Println("The current directory is not a git repository. Consider using Qovery within a git project")
			os.Exit(1)
		}

		p := util.QoveryYML{}

		// check the user is auth; if not then exit
		if api.GetAccount().Id == "" {
			fmt.Println("You are not authenticated. Authenticate yourself with 'qovery auth' before using 'qovery init'!")
			os.Exit(1)
		}

		fmt.Print(util.AsciiName)

		fmt.Println("Reply to the following questions to initialize Qovery for this application")
		fmt.Println("For more info: " + color.New(color.Bold).Sprint("https://docs.qovery.com"))

		fmt.Println(AskForTemplate())

		p.Application.Name = CurrentDirectoryName()

		for {
			p.Application.Project = AskForProject()
			p.Application.CloudRegion = AskForCloudRegion()

			if p.Application.Project != "" && p.Application.CloudRegion != "" {
				break
			}

			// Should not happened
			fmt.Println("Form is incomplete... Try again")
		}

		p.Application.PubliclyAccessible = true //util.AskForConfirmation(false, "Would you like to make your application publicly accessible?", "y") TODO

		if p.Application.PubliclyAccessible {
			p.Routers = []util.QoveryYMLRouter{
				{
					Name: "main",
					Routes: []util.QoveryYMLRoute{
						{
							ApplicationName: p.Application.Name,
							Paths:           []string{"/*"},
						},
					},
				},
			}
			// TODO
			// p.Routers.DNS = util.AskForInput(true, "Do you want to set a custom domain (ex: api.foo.com)?")
		}

		count := 1
		for {
			if askForAddDatabase(count) {
				db := AddDatabaseWizard()
				if db != nil {
					p.Databases = append(p.Databases, *db)
				}
			} else {
				break
			}

			count++
		}

		yaml, err := yaml.Marshal(&p)
		if err != nil {
			log.Fatalln(err)
		}

		f, err := os.Create(".qovery.yml")
		if err != nil {
			log.Fatalln(err)
		}

		_, err = f.Write(yaml)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(color.GreenString("✓") + " Your Qovery configuration file has been successfully created (.qovery.yml)")

		fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("\n!!! IMPORTANT !!!"))
		fmt.Println(color.YellowString("Qovery needs to get access to your git repository"))
		fmt.Println("https://github.com/apps/qovery/installations/new")

		prompt := promptui.Select{
			Label: "Would you like to open the link above?",
			Size:  2,
			Items: []string{"No", "Yes"},
		}

		openLink, _, _ := prompt.Run()
		if openLink == 1 {
			_ = browser.OpenURL("https://github.com/apps/qovery/installations/new")
		}

		fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("\n!!! IMPORTANT !!!"))
		fmt.Println("1/ Commit and push the \".qovery.yml\" file to get your app deployed")
		fmt.Println("➤ Run: git add .qovery.yml && git commit -m \"add .qovery.yml\" && git push -u origin master")
		fmt.Println("2/ Check the status of your deployment")
		fmt.Println("➤ Run: qovery status")
		fmt.Println("\nEnjoy! 👋")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func askForAddDatabase(count int) bool {
	question := "Do you need a database? (PostgreSQL, MongoDB, MySQL, ...)"
	if count > 1 {
		question = "Do you want to add another database?"
	}

	prompt := promptui.Select{
		Label: question,
		Size:  2,
		Items: []string{"No", "Yes"},
	}

	result, _, _ := prompt.Run()

	if result == 0 {
		return false
	}

	return true
}

func AskForTemplate() string {
	prompt := promptui.Select{
		Label: "Do you want to use a Dockerfile template? (NodeJS, Java, PHP, Python...)",
		Size:  2,
		Items: []string{"Yes", "No"},
	}

	_, result, _ := prompt.Run()

	templateName := ""
	if result == "Yes" {
		prompt = promptui.Select{
			Label: "Choose the template you want",
			Size:  50,
			Items: []string{"Node", "Java", "Python", "Hasura"},
		}

		_, result, _ := prompt.Run()
		templateName = result
	}

	return templateName
}

func AskForProject() string {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := api.ListProjects().Results

	var projectNames []string
	for _, v := range projects {
		projectNames = append(projectNames, v.Name)
	}

	sort.Strings(projectNames)

	projectTypeChoice := 1
	if len(projectNames) > 0 {
		prompt := promptui.Select{
			Label: "I want to add this application to...",
			Size:  2,
			Items: []string{
				"An existing project",
				"A new project",
			},
		}

		projectTypeChoice, _, _ = prompt.Run()
	}

	var projectName string
	if projectTypeChoice == 1 {
		for {
			prompt := promptui.Prompt{Label: "Enter the project name"}

			projectName, _ = prompt.Run()
			if api.GetProjectByName(projectName).Id == "" {
				break
			}

			fmt.Printf("This project name (%s) already exists, please choose another one\n", projectName)
		}
	} else {
		// select an existing project
		prompt := promptui.Select{
			Label: "Choose the project you want",
			Size:  len(projectNames),
			Items: projectNames,
		}

		_, projectName, _ = prompt.Run()
	}

	return projectName
}

func AskForCloudRegion() string {
	clouds := api.ListCloudProviders().Results

	keyByDescription := make(map[string]string)
	var names []string

	for _, c := range clouds {
		for _, r := range c.Regions {
			key := fmt.Sprintf("%s/%s", c.Name, r.FullName)
			name := fmt.Sprintf("%s | %s (%s)", strings.ToUpper(c.Name), r.Description, key)
			names = append(names, name)
			keyByDescription[name] = key
		}
	}

	sort.Strings(names)

	prompt := promptui.Select{
		Label: "Choose the region where you want to host your project and applications",
		Size:  len(names),
		Items: names,
	}

	_, nameChoice, _ := prompt.Run()

	return keyByDescription[nameChoice]
}

func AddDatabaseWizard() *util.QoveryYMLDatabase {

	choices := []string{"PostgreSQL", "MongoDB", "MySQL"}

	prompt := promptui.Select{
		Label: "Choose the database you need",
		Size:  len(choices),
		Items: choices,
	}

	_, choice, _ := prompt.Run()

	var versionChoices []string
	switch choice {
	case "PostgreSQL":
		versionChoices = []string{"latest", "11.5", "11.4", "11.2", "11.1", "10.10", "9.6"}
	case "MongoDB":
		versionChoices = []string{"latest", "3.6"}
	case "MySQL":
		versionChoices = []string{"latest", "8.0", "5.7", "5.6", "5.5"}
	default:
		versionChoices = []string{}
	}

	prompt = promptui.Select{
		Label: fmt.Sprintf("Choose the %s version you want", color.New(color.Bold).Sprint(choice)),
		Size:  len(versionChoices),
		Items: versionChoices,
	}

	_, versionChoice, _ := prompt.Run()
	if versionChoice == "latest" {
		versionChoice = versionChoices[1]
	}

	name := fmt.Sprintf("my-%s-%d", strings.ToLower(choice), util.RandomInt())

	return &util.QoveryYMLDatabase{Name: name, Type: strings.ToLower(choice), Version: versionChoice}
}

func CurrentDirectoryName() string {
	currentDirectoryPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	s := strings.Split(currentDirectoryPath, string(os.PathSeparator))

	return s[len(s)-1]
}

func intPointerValue(i *int) string {
	if i == nil {
		return "0"
	}
	return strconv.Itoa(*i)
}
