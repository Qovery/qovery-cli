package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Template struct {
	Content string
}

func GetDockerfileTemplate(templateName string) Template {
	return getFileTemplate(templateName, "Dockerfile")

}

func GetQoveryConfigTemplate(templateName string) Template {
	return getFileTemplate(templateName, ".qovery.yml")
}

func getFileTemplate(projectName string, fileName string) Template {
	resp, err := http.Get("https://raw.githubusercontent.com/Qovery/qovery-templates/master/" + projectName + "/" + fileName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("Template for " + projectName + " not found.")
		fmt.Println("Check the list of available templates in the templates repository:")
		fmt.Println("https://github.com/Qovery/qovery-templates")
		os.Exit(1)
	}

	defer resp.Body.Close()

	templateBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	templateContent := string(templateBytes)

	return Template{Content: templateContent}
}
