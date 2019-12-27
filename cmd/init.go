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

		project := AskForProject()
		repository := AskForRepository(project)

		p.Qovery.Key = fmt.Sprintf("%s/%s/%s", api.GetAccountId(), project.Id, repository.Id)
		p.Application.Project = project.Name
		p.Application.Name = repository.Name
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
		fmt.Println("https://github.com/apps/qovery/installations/new/permissions?target_id=55960755")

		openLink := util.AskForConfirmation(false, "Would you like to open the link above?", "n")
		if openLink {
			_ = browser.OpenURL("https://github.com/apps/qovery/installations/new/permissions?target_id=55960755")
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
	f, _ := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

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

func AskForProject() api.Project {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := api.ListProjects().Results

	var projectNames []string
	for _, v := range projects {
		projectNames = append(projectNames, v.Name)
	}

	sort.Strings(projectNames)

	choice := "create a new project"
	if len(projectNames) > 1 {
		choice = util.AskForSelect([]string{"create a new project", "select an existing project"}, "What do you want?", "create a new project")
	}

	if choice == "create a new project" {
		var name string
		for {
			name = util.AskForInput(false, "Enter the project name")
			if api.GetProjectByName(name) == nil {
				break
			}

			fmt.Printf("This project name (%s) already exists, please choose another one\n", name)
		}

		region := AskForCloudRegion()
		return api.CreateProject(api.Project{Name: name, CloudProviderRegion: region})
	} else {
		// select an existing project
		choice = util.AskForSelect(projectNames, "Choose the project you want", "")
		if choice == "" {
			return AskForProject()
		}
	}

	for _, v := range projects {
		if v.Name == choice {
			return v
		}
	}

	return api.Project{}
}

func AskForCloudRegion() api.CloudProviderRegion {
	clouds := api.ListCloudProviders().Results

	var names []string
	for _, c := range clouds {
		for _, r := range c.Regions {
			names = append(names, fmt.Sprintf("%s/%s", c.Name, r.FullName))
		}
	}

	sort.Strings(names)

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

func AskForRepository(project api.Project) api.Repository {
	currentDirectoryName := CurrentDirectoryName()

	// check if the application name already exists and ask to confirm if it does exist
	// if it does not exist, then create it to get the ID

	// select repository from existing ones or ask to create a new one; then take the ID
	repositories := api.ListRepositories(project.Id).Results

	var repoNames = []string{"Create new application"}
	for _, v := range repositories {
		repoNames = append(repoNames, v.Name)
	}

	choice := "Create new application"
	if len(repoNames) > 1 {
		choice = util.AskForSelect(repoNames, "Choose the application you want (or create a new one)", "Create new application")
	}

	if choice == "Create new application" {
		var name string
		for {
			name = util.AskForInput(true, fmt.Sprintf("Enter the application name [default: %s]", currentDirectoryName))

			if name == "" {
				name = currentDirectoryName
			}

			if api.GetRepositoryByName(project.Id, name) == nil {
				break
			}

			fmt.Printf("This application name (%s) already exists, please choose another one\n", name)
		}

		remoteURLs := util.ListRemoteURLs()
		remoteURL := ""

		if len(remoteURLs) == 1 {
			remoteURL = remoteURLs[0]
		} else if len(remoteURLs) > 1 {
			// ask for selecting the remote
			remoteURL = util.AskForSelect(remoteURLs, "Choose the git repository URL", "")
		}

		if remoteURL == "" {
			remoteURL = util.AskForInput(false, "Enter the git repository URL")
		}

		return api.CreateRepository(project.Id, api.Repository{Name: name, URL: remoteURL})
	}

	for _, r := range repositories {
		if r.Name == choice {
			return r
		}
	}

	return api.Repository{}
}

func AddDatabaseWizard() *util.QoveryYMLDatabase {

	//choices := []string{"PostgreSQL", "MongoDB", "MySQL", "Redis", "Memcached", "Elasticsearch"}
	choices := []string{"PostgreSQL", "MySQL"}

	choice := util.AskForSelect(choices, "Choose the database you want to add", "")
	if choice == "" {
		return nil
	}

	var versionChoices []string
	switch choice {
	case "PostgreSQL":
		versionChoices = []string{"latest", "12", "11.5", "11.4", "11.2", "11.1", "10.10", "9.6"}
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
