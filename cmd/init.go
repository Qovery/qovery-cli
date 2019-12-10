package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
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

		p.Application.Project = util.AskForInput(false, "Enter the project name")
		p.Application.Name = util.AskForInput(false, "Enter the application name")
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

func AddDatabaseWizard() *util.QoveryYMLDatabase {

	choices := []string{"PostgreSQL", "MongoDB", "MySQL", "Redis", "Memcached", "Elasticsearch"}

	choice := util.AskForSelect(choices, "Choose the database you want to add", "")
	if choice == "" {
		return nil
	}

	name := util.AskForInput(false, "Set the database name")

	return &util.QoveryYMLDatabase{Name: name, Type: strings.ToLower(choice)}
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

	name := util.AskForInput(false, "Set the broker name")

	return &util.QoveryYMLBroker{Name: name, Type: strings.ToLower(choice)}
}
