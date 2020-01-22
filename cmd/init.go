package cmd

import (
	"fmt"
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

		// TODO check Dockerfile

		p := util.QoveryYML{}

		// check the user is auth; if not then exit
		if api.GetAccount().Id == "" {
			fmt.Println("You are not authenticated. Authenticate yourself with 'qovery auth' before using 'qovery init'!")
			os.Exit(1)
		}

		fmt.Println("Reply to the following questions to initialize Qovery for this application")
		fmt.Println("For more info: https://docs.qovery.com")

		p.Application.Project = AskForProject()
		p.Application.Name = CurrentDirectoryName()
		p.Application.CloudRegion = AskForCloudRegion()
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
		for count < 20 {
			addDatabase := false
			if count == 1 {
				addDatabase = askForAddDatabase(true)
			} else {
				addDatabase = askForAddDatabase(false)
			}

			if addDatabase {
				db := AddDatabaseWizard()
				if db != nil {
					p.Databases = append(p.Databases, *db)
				}
			} else {
				break
			}

			count++
		}

		/** TODO
		count = 1
		for count < 20 {
			addBroker := false
			if count == 1 {
				addBroker = askForAddBroker(true)
			} else {
				addBroker = askForAddBroker(false)
			}

			if addBroker {
				b := AddBrokerWizard()
				if b != nil {
					p.Brokers = append(p.Brokers, *b)
				}
			} else {
				break
			}

			count++
		}
		*/

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

		addLinesToGitIgnore()

		fmt.Println("✓ Your Qovery configuration file has been successfully created (.qovery.yml)")

		fmt.Println("\n!!!IMPORTANT!!!")
		fmt.Println("Qovery needs to get access to your git repository")
		fmt.Println("https://github.com/apps/qovery/installations/new")

		openLink := util.AskForConfirmation(false, "Would you like to open the link above?", "n")
		if openLink {
			_ = browser.OpenURL("https://github.com/apps/qovery/installations/new")
		}

		fmt.Println("\n!!!IMPORTANT!!!")
		fmt.Println("1/ Commit and push the \".qovery.yml\" file to get your app deployed")
		fmt.Println("➤ Run: git add .qovery.yml && git commit -m \"add .qovery.yml\" && git push -u origin master")
		fmt.Println("\n2/ Check the status of your deployment")
		fmt.Println("➤ Run: qovery status")
		fmt.Println("\nEnjoy! 👋")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func addLinesToGitIgnore() {
	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("fail to create/upsert file .gitignore: %s", err.Error())
	}

	defer f.Close()
	_, _ = f.WriteString("\n.qovery\nlocal_configuration.json\n")
}

func askForAddDatabase(firstTime bool) bool {
	if firstTime {
		return util.AskForConfirmation(false, "Do you need a database? (PostgreSQL, MySQL, MongoDB, ...)", "n")
	} else {
		return util.AskForConfirmation(false, "Do you want to add another database?", "n")
	}
}

func AskForProject() string {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := api.ListProjects().Results

	var projectNames []string
	for _, v := range projects {
		projectNames = append(projectNames, v.Name)
	}

	sort.Strings(projectNames)

	choice := "create a new project"
	if len(projectNames) > 0 {
		choice = util.AskForSelect([]string{"create a new project", "select an existing project"}, "What do you want?", "create a new project")
	}

	if choice == "create a new project" {
		var name string
		for {
			name = util.AskForInput(false, "Enter the project name")
			if api.GetProjectByName(name).Id == "" {
				break
			}

			fmt.Printf("This project name (%s) already exists, please choose another one\n", name)
		}

		return name
	} else {
		// select an existing project
		choice = util.AskForSelect(projectNames, "Choose the project you want", "")
		if choice == "" {
			return AskForProject()
		}
	}

	return choice
}

func AskForCloudRegion() string {
	clouds := api.ListCloudProviders().Results

	var names []string
	for _, c := range clouds {
		for _, r := range c.Regions {
			names = append(names, fmt.Sprintf("%s/%s", c.Name, r.FullName))
		}
	}

	sort.Strings(names)

	choice := util.AskForSelect(names, "Choose the region where you want to host your project and applications", "")

	return choice
}

func AddDatabaseWizard() *util.QoveryYMLDatabase {

	//choices := []string{"PostgreSQL", "MongoDB", "MySQL", "Redis", "Memcached", "Elasticsearch"}
	choices := []string{"PostgreSQL"}

	choice := util.AskForSelect(choices, "Choose the database you want to add", "")
	if choice == "" {
		return nil
	}

	var versionChoices []string
	switch choice {
	case "PostgreSQL":
		versionChoices = []string{"latest", "11.5", "11.4", "11.2", "11.1", "10.10", "9.6"}
	case "MongoDB":
		versionChoices = []string{"latest", "3.6"}
	case "MySQL":
		versionChoices = []string{"latest", "8.0", "5.7", "5.6", "5.5"}
	case "Redis":
		versionChoices = []string{"latest", "5.0", "4.0", "3.2", "2.8", "2.6"}
	case "Memcached":
		versionChoices = []string{"latest", "1.5", "1.4"}
	case "Elasticsearch":
		versionChoices = []string{"latest", "7.1", "6.8", "5.6", "2.3", "1.5"}
	default:
		versionChoices = []string{}
	}

	versionChoice := util.AskForSelect(versionChoices, fmt.Sprintf("Choose the %s version you want", choice), "latest")
	if versionChoice == "latest" {
		versionChoice = versionChoices[1]
	}

	name := fmt.Sprintf("my-%s-%d", strings.ToLower(choice), util.RandomInt())

	return &util.QoveryYMLDatabase{Name: name, Type: strings.ToLower(choice), Version: versionChoice}
}

func askForAddBroker(firstTime bool) bool {
	if firstTime {
		return util.AskForConfirmation(false, "Do you need a broker? (RabbitMQ, Kafka, ...)", "n")
	} else {
		return util.AskForConfirmation(false, "Do you want to add another broker?", "n")
	}
}

func AddBrokerWizard() *util.QoveryYMLBroker {

	choices := []string{"RabbitMQ", "Kafka"}

	choice := util.AskForSelect(choices, "Choose the broker you want to add", "")
	if choice == "" {
		return nil
	}

	var versionChoices []string
	switch choice {
	case "RabbitMQ":
		versionChoices = []string{"latest", "3.8", "3.7", "3.6"}
	case "Kafka":
		versionChoices = []string{"latest", "2.3", "2.2", "2.1"}
	default:
		versionChoices = []string{}
	}

	versionChoice := util.AskForSelect(versionChoices, fmt.Sprintf("Choose the %s version you want", choice), "latest")
	if versionChoice == "latest" {
		versionChoice = versionChoices[1]
	}

	name := fmt.Sprintf("my-%s-%d", strings.ToLower(choice), util.RandomInt())

	return &util.QoveryYMLBroker{Name: name, Type: strings.ToLower(choice), Version: versionChoice}
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
