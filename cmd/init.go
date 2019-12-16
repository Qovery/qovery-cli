package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"qovery.go/api"
	"qovery.go/util"
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

		// TODO check Dockerfile

		p := util.QoveryYML{}

		// check the user is auth; if not then exit
		if api.GetAccountId() == "" && api.GetAccount().Id == "" {
			fmt.Println("You must use 'qovery auth' before using 'qovery init'!")
			os.Exit(1)
		}

		project := AskForProject()
		p.Application.Project = project.Name

		currentDirectoryName := currentDirectoryName()

		p.Application.Name = util.AskForInput(true, fmt.Sprintf("Enter the application name [default: %s]", currentDirectoryName))
		// check if the application name already exists and ask to confirm if it does exist
		// if it does not exist, then create it to get the ID
		// TODO

		if p.Application.Name == "" {
			p.Application.Name = currentDirectoryName
		}

		p.Application.PubliclyAccessible = util.AskForConfirmation(false, "Would you like to expose publicly your application?", "y")

		if p.Application.PubliclyAccessible {
			p.Network.DNS = util.AskForInput(true, "Do you want to set a custom domain (ex: api.foo.com)?")
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

		count = 1

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

		fmt.Println("✓ Your Qovery configuration file has been successfully created (.qovery.yml)")
		fmt.Println("✓ Commit into your repository and push it to get this deployed")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func askForAddDatabase(firstTime bool) bool {
	if firstTime {
		return util.AskForConfirmation(false, "Do you need a database? (PostgreSQL, MySQL, MongoDB, ...)", "n")
	} else {
		return util.AskForConfirmation(false, "Do you want to add another database?", "n")
	}
}

func AskForProject() api.Project {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := api.ListProjects().Results

	var projectNames = []string{"Create new project"}
	for _, v := range projects {
		projectNames = append(projectNames, v.Name)
	}

	choice := "Create new project"
	if len(projectNames) > 0 {
		choice = util.AskForSelect(projectNames, "Choose the project you want (or create a new one)", "Create new project")
	}

	if choice == "Create new project" {
		name := util.AskForInput(false, "Enter the project name")
		region := AskForCloudRegions()
		return api.CreateProject(api.Project{Name: name, CloudProviderRegion: region})
	}

	for _, v := range projects {
		if v.Name == choice {
			return v
		}
	}

	return api.Project{}
}

func AskForCloudRegions() api.CloudProviderRegion {
	clouds := api.ListCloudProviders().Results

	var names []string
	for _, c := range clouds {
		for _, r := range c.Regions {
			names = append(names, fmt.Sprintf("%s/%s", c.Name, r.FullName))
		}
	}

	choice := util.AskForSelect(names, "Choose the region where you want to host your project and applications", "")

	for _, c := range clouds {
		for _, r := range c.Regions {
			choiceName := fmt.Sprintf("%s/%s", c.Name, r.FullName)
			if choiceName == choice {
				return r
			}
		}
	}

	return api.CloudProviderRegion{}
}

func AddDatabaseWizard() *util.QoveryYMLDatabase {

	choices := []string{"PostgreSQL", "MongoDB", "MySQL", "Redis", "Memcached", "Elasticsearch"}

	choice := util.AskForSelect(choices, "Choose the database you want to add", "")
	if choice == "" {
		return nil
	}

	var versionChoices []string
	switch choice {
	case "PostgreSQL":
		versionChoices = []string{"12", "11.5", "11.4", "11.2", "11.1", "10.10", "9.6"}
	case "MongoDB":
		versionChoices = []string{"3.6"}
	case "MySQL":
		versionChoices = []string{"8.0", "5.7", "5.6", "5.5"}
	case "Redis":
		versionChoices = []string{"5.0", "4.0", "3.2", "2.8", "2.6"}
	case "Memcached":
		versionChoices = []string{"1.5", "1.4"}
	case "Elasticsearch":
		versionChoices = []string{"7.1", "6.8", "5.6", "2.3", "1.5"}
	default:
		versionChoices = []string{}
	}

	versionChoice := util.AskForSelect(versionChoices, fmt.Sprintf("Choose the %s version you want", choice), "")
	if versionChoice == "" {
		return nil
	}

	name := util.AskForInput(false, "Set the database name")

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
		versionChoices = []string{"3.8", "3.7", "3.6"}
	case "Kafka":
		versionChoices = []string{"2.3", "2.2", "2.1"}
	default:
		versionChoices = []string{}
	}

	versionChoice := util.AskForSelect(versionChoices, fmt.Sprintf("Choose the %s version you want", choice), "")
	if versionChoice == "" {
		return nil
	}

	name := util.AskForInput(false, "Set the broker name")

	return &util.QoveryYMLBroker{Name: name, Type: strings.ToLower(choice), Version: versionChoice}
}

func currentDirectoryName() string {
	currentDirectoryPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	s := strings.Split(currentDirectoryPath, string(os.PathSeparator))

	return s[len(s)-1]
}