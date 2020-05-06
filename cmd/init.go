package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"qovery.go/io"
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
		io.DownloadSource(templateFlag)
		filepath.Walk(templateFlag, replaceAppName)
		projectTemplate := io.GetTemplate(templateFlag)
		p.Application.Name = templateFlag
		p.Application.Project = askForProject()
		p.Application.CloudRegion = askForCloudRegion()
		p.Routers = projectTemplate.QoveryYML.Routers
		p.Databases = projectTemplate.QoveryYML.Databases
		for routerIdx, router := range p.Routers {
			for routeIdx := range router.Routes {
				// change route application name
				p.Routers[routerIdx].Routes[routeIdx].ApplicationName = p.Application.Name
			}
		}

		writeFiles(projectTemplate, p)
		printFinalMessage(projectTemplate)

		os.Exit(0)
	}

	fmt.Println("Reply to the following questions to initialize Qovery for this application")
	fmt.Println("For more info: " + color.New(color.Bold).Sprint("https://docs.qovery.com"))

	projectTemplate := askForTemplate()

	p.Application.Name = currentDirectoryName()

	count := 0
	for {
		p.Application.Project = askForProject()
		p.Application.CloudRegion = askForCloudRegion()

		if p.Application.Project != "" && p.Application.CloudRegion != "" {
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

	printFinalMessage(projectTemplate)
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

	if result == 0 {
		return false
	}

	return true
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

func askForProject() string {
	// select project from existing ones or ask to create a new one; then take the ID
	projects := io.ListProjects().Results

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
			if io.GetProjectByName(projectName).Id == "" {
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

func askForCloudRegion() string {
	clouds := io.ListCloudProviders().Results

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
		versionChoices = []string{"latest", "11.7", "11.6", "11.5", "10.10", "9.6"}
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

func replaceAppName(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if !!fi.IsDir() {
		return nil //
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
