package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"qovery.go/io"
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
		runInit()
	},
}

var templateFlag string

func init() {
	initCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "project template")
	RootCmd.AddCommand(initCmd)
}

func runInit() {
	if _, err := os.Stat(".qovery.yml"); err == nil {
		fmt.Println("You already have a .qovery.yml file")

		prompt := promptui.Select{
			Label: color.YellowString("Do you want to overwrite it?"),
			Size:  2,
			Items: []string{"No", "Yes"},
		}

		choice, _, _ := prompt.Run()
		if choice == 0 {
			os.Exit(0)
		}
	}

	p := io.QoveryYML{}

	// check the user is auth; if not then exit
	if io.GetAccount().Id == "" {
		fmt.Println("You are not authenticated. Authenticate yourself with 'qovery auth' before using 'qovery init'!")
		os.Exit(1)
	}

	fmt.Print(io.AsciiName)

	if templateFlag != "" {
		projectTemplate := io.GetTemplate(templateFlag)
		p.Application.Name = templateFlag
		p.Application.Organization = askForOrganization()
		p.Application.Project = askForProject(p.Application.Organization)
		p.Routers = projectTemplate.QoveryYML.Routers
		p.Databases = projectTemplate.QoveryYML.Databases
		for routerIdx, router := range p.Routers {
			for routeIdx := range router.Routes {
				// change route application name
				p.Routers[routerIdx].Routes[routeIdx].ApplicationName = p.Application.Name
			}
		}

		io.DownloadSource(templateFlag)
		err := filepath.Walk(templateFlag, replaceAppName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		writeFiles(projectTemplate, p)
		err = io.InitializeEmptyGitRepository(templateFlag)
		if err != nil {
			fmt.Println("Could not initialize an empty Git repository in " + templateFlag + " folder")
			os.Exit(1)
		}

		askForGithubPermissions()
		printFinalMessageForTemplatedProject(projectTemplate)
		os.Exit(0)
	} else {
		templateFlag = "."
	}

	fmt.Println("Reply to the following questions to initialize Qovery for this application")
	fmt.Println("For more info: " + color.New(color.Bold).Sprint("https://docs.qovery.com"))

	projectTemplate := askForTemplate()

	p.Application.Name = currentDirectoryName()

	count := 0
	for {
		p.Application.Organization = askForOrganization()

		if p.Application.Organization != "" {
			break
		}

		p.Application.Project = askForProject(p.Application.Organization)

		if p.Application.Project != "" {
			break
		}

		// Should not happened
		fmt.Println("Form is incomplete... Try again")
		count++

		if count >= 2 {
			os.Exit(0)
		}
	}

	if projectTemplate.Name == "" {
		p.Application.PubliclyAccessible = true // TODO change this
	}

	if p.Application.PubliclyAccessible && projectTemplate.Name == "" {
		p.Routers = []io.QoveryYMLRouter{
			{
				Name: "main",
				Routes: []io.QoveryYMLRoute{
					{
						ApplicationName: p.Application.Name,
						Paths:           []string{"/"},
					},
				},
			},
		}
	} else if projectTemplate.Name != "" {
		p.Routers = projectTemplate.QoveryYML.Routers
	}

	for routerIdx, router := range p.Routers {
		for routeIdx := range router.Routes {
			// change route application nam
			p.Routers[routerIdx].Routes[routeIdx].ApplicationName = p.Application.Name
		}
	}

	// TODO
	// p.Routers.DNS = io.AskForInput(true, "Do you want to set a custom domain (ex: api.foo.com)?")

	if len(projectTemplate.QoveryYML.Databases) > 0 {
		// add databases from template
		p.Databases = projectTemplate.QoveryYML.Databases

		databaseWord := "database"
		if len(p.Databases) > 1 {
			databaseWord = "databases"
		}

		fmt.Println(color.GreenString("%s has configured %d %s", projectTemplate.Name, len(p.Databases), databaseWord))
		for _, db := range p.Databases {
			fmt.Println(color.GreenString("✓") + fmt.Sprintf(" database: %s version: %s", db.Type, db.Version))
		}

	}

	for {
		if askForAddDatabase(len(p.Databases)) {
			db := addDatabaseWizard()
			if db != nil {
				p.Databases = append(p.Databases, *db)
				fmt.Println(color.GreenString("✓") + fmt.Sprintf(" database: %s version: %s", db.Type, db.Version))
			}
		} else {
			break
		}
	}

	writeFiles(projectTemplate, p)

	fmt.Println(color.GreenString("✓") + " Your Qovery configuration file has been successfully created (.qovery.yml)")
	fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("\n!!! IMPORTANT !!!"))
	askForGithubPermissions()

	printFinalMessage(projectTemplate)
}

func askForGithubPermissions() {
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
}

func askForAddDatabase(count int) bool {
	question := "Do you need a database? (PostgreSQL, MongoDB, MySQL, ...)"
	if count >= 1 {
		question = "Do you want to add another database?"
	}

	prompt := promptui.Select{
		Label: question,
		Size:  2,
		Items: []string{"No", "Yes"},
	}

	result, _, _ := prompt.Run()

	report := true
	if result == 0 {
		report = false
	}

	return report
}

func askForTemplate() io.Template {
	prompt := promptui.Select{
		Label: "Do you want to use a Dockerfile template? (NodeJS, Java, PHP, Python...)",
		Size:  2,
		Items: []string{"Yes", "No"},
	}

	x, _, _ := prompt.Run()
	if x == 1 {
		return io.Template{}
	}

	templates := io.ListAvailableTemplates()

	var templateNames []string
	for _, template := range templates {
		templateNames = append(templateNames, template.ToString())
	}

	prompt = promptui.Select{
		Label: "Choose the template you want",
		Size:  50,
		Items: templateNames,
	}

	choice, _, _ := prompt.Run()
	templateName := templates[choice].Name

	return io.GetTemplate(templateName)
}

func askForOrganization() string {
	// select organizations from existing ones or ask to create a new one; then take the ID
	organizations := io.ListOrganizations().Results

	if len(organizations) == 1 {
		return organizations[0].DisplayName
	} else if len(organizations) == 0 {
		return "QoveryCommunity"
	}

	var organizationNames []string
	for _, v := range organizations {
		organizationNames = append(organizationNames, v.Name)
	}

	sort.Strings(organizationNames)

	prompt := promptui.Select{
		Label: "Choose the organization you want",
		Size:  len(organizationNames),
		Items: organizationNames,
	}

	_, organizationName, _ := prompt.Run()

	return organizationName
}

func askForProject(organizationName string) string {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := io.ListProjects(organizationName).Results

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
			if io.GetProjectByName(projectName, organizationName).Id == "" {
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

func addDatabaseWizard() *io.QoveryYMLDatabase {

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
		versionChoices = []string{"latest", "12", "11", "10", "9"}
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

	name := fmt.Sprintf("my-%s-%d", strings.ToLower(choice), io.RandomInt())

	return &io.QoveryYMLDatabase{Name: name, Type: strings.ToLower(choice), Version: versionChoice}
}

func currentDirectoryName() string {
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

func writeFiles(template io.Template, p io.QoveryYML) {
	yamlContent, err := yaml.Marshal(&p)
	if err != nil {
		log.Fatalln(err)
	}

	// create .qovery.yml
	os.Remove(templateFlag + string(os.PathSeparator) + ".qovery.yml")
	f, err := os.Create(templateFlag + string(os.PathSeparator) + ".qovery.yml")
	if err != nil {
		log.Fatalln(err)
	}

	_, err = f.Write(yamlContent)
	if err != nil {
		log.Fatalln(err)
	}

	if template.DockerfileContent != "" {
		// create Dockerfile
		os.Remove(templateFlag + string(os.PathSeparator) + "Dockerfile")
		f, err := os.Create(templateFlag + string(os.PathSeparator) + "Dockerfile")
		if err != nil {
			log.Fatalln(err)
		}

		_, err = f.Write([]byte(template.DockerfileContent))
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func printFinalMessage(template io.Template) {
	fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("\n!!! IMPORTANT !!!"))
	fmt.Println(color.New(color.Bold).Sprint("1/ Commit and push the \".qovery.yml\" file to get your app deployed"))
	fmt.Println("➤ Run: git add .qovery.yml Dockerfile && git commit -m \"add .qovery.yml\" && git push -u origin master")
	fmt.Println(color.New(color.Bold).Sprint("2/ Check the status of your deployment"))
	fmt.Println("➤ Run: qovery status")

	if len(template.Commands) > 0 {
		fmt.Println(color.New(color.Bold).Sprint("3/ Execute the following commands"))
		for _, command := range template.Commands {
			fmt.Println(fmt.Sprintf("➤ Run: %s", command))
		}
	}

	fmt.Println("\nEnjoy! 👋")
}

func printFinalMessageForTemplatedProject(template io.Template) {
	fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("\n!!! IMPORTANT !!!"))
	fmt.Println(color.New(color.Bold).Sprint("1/ Navigate to your new application: cd " + templateFlag))
	fmt.Println(color.New(color.Bold).Sprint("2/ Push the code to a new repository on Github"))
	fmt.Println(color.New(color.Bold).Sprint("3/ Run: `qovery status` to see the status of app deployment"))

	if len(template.Commands) > 0 {
		fmt.Println(color.New(color.Bold).Sprint("4/ Execute the following commands"))
		for _, command := range template.Commands {
			fmt.Println(fmt.Sprintf("➤ Run: %s", command))
		}
	}

	fmt.Println("\nEnjoy! 👋")
}

func replaceAppName(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return nil
	}

	read, err := ioutil.ReadFile(path)

	if err != nil {
		panic(err)
	}

	newContents := strings.Replace(string(read), "${APP_NAME}", templateFlag, -1)

	err = ioutil.WriteFile(path, []byte(newContents), 0)
	if err != nil {
		panic(err)
	}

	return nil
}
