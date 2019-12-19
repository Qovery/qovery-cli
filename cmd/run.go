package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"qovery.go/api"
	"qovery.go/util"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Equivalent to 'docker build' and 'docker run' but with Qovery magic sauce",
	Long: `RUN performs 'docker build' and 'docker run' actions and set Qovery properties to target the right environment . For example:

	qovery run`,
	Run: func(cmd *cobra.Command, args []string) {
		qYML := util.CurrentQoveryYML()
		branchName := util.CurrentBranchName()
		projectName := qYML.Application.Project

		if branchName == "" || projectName == "" {
			fmt.Println("The current directory is not a Qovery project. Please consider using 'qovery init'")
			os.Exit(1)
		}

		// TODO check docker is running
		dockerClient, err := client.NewEnvClient()
		if err != nil {
			fmt.Println("Is Docker installed?")
			panic(err)
		}

		qConf := util.CurrentQoveryYML()
		appName := qConf.Application.Name

		project := api.GetProjectByName(projectName)
		if project == nil {
			fmt.Println("The project does not exist. Are you well authenticated with the right user? Do 'qovery auth' to be sure")
			os.Exit(1)
		}

		applications := api.ListApplicationsRaw(project.Id, branchName)
		if val, ok := applications["results"]; ok {
			results := val.([]interface{})
			for _, application := range results {
				a := application.(map[string]interface{})
				if a["name"] == appName {

					ReloadEnvironment()
					image := buildContainer(dockerClient, qYML.Application.DockerfilePath())
					runContainer(dockerClient, image, a) // TODO docker file content

					break
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
}

func buildContainer(client *client.Client, dockerfilePath string) *types.ImageSummary {
	tar := archiver.Tar{MkdirAll: true}

	buildTarPath := filepath.FromSlash(fmt.Sprintf("%s/build.tar", os.TempDir()))

	_ = os.Remove(buildTarPath)
	err := tar.Archive([]string{"."}, buildTarPath)

	if err != nil {
		panic(err)
	}

	f, err := os.Open(buildTarPath)
	if err != nil {
		panic(err)
	}

	r, err := client.ImageBuild(context.Background(), f, types.ImageBuildOptions{Dockerfile: dockerfilePath})
	defer r.Body.Close()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_ = writeToLog(r.Body)

	images, err := client.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	// last created image // TODO change this, it is not good
	image := images[0]

	return &image
}

func runContainer(client *client.Client, image *types.ImageSummary, configurationMap map[string]interface{}) {
	j, _ := json.Marshal(configurationMap)
	configurationMapB64 := base64.StdEncoding.EncodeToString(j)

	config := &container.Config{Image: image.ID, Env: []string{fmt.Sprintf("QOVERY_JSON_B64=%s", configurationMapB64)}}
	hostConfig := &container.HostConfig{}

	exposePorts := util.ExposePortsFromCurrentDockerfile()

	// TODO add all ports and not only the last one exposed
	for _, exposePort := range exposePorts {
		portTCP := nat.Port(fmt.Sprintf("%s/tcp", exposePort))
		config.ExposedPorts = nat.PortSet{portTCP: struct{}{}}
		hostConfig.PortBindings = map[nat.Port][]nat.PortBinding{portTCP: {{HostIP: "0.0.0.0", HostPort: exposePort}}}
	}

	c, err := client.ContainerCreate(context.Background(), config, hostConfig, nil, "")
	if err != nil {
		panic(err)
	}

	_ = client.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{})

	go func() {
		containerLogsOptions := types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		}

		out, err := client.ContainerLogs(context.Background(), c.ID, containerLogsOptions)

		if err != nil {
			panic(err)
		}

		_, _ = io.Copy(os.Stdout, out)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			// sig is a ^C, handle it
			_ = client.ContainerStop(context.Background(), c.ID, nil)
		}
	}()

	_, _ = client.ContainerWait(context.Background(), c.ID)
}

func writeToLog(reader io.ReadCloser) error {
	defer reader.Close()
	rd := bufio.NewReader(reader)

	for {
		n, _, err := rd.ReadLine()
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		var m map[string]string
		_ = json.Unmarshal(n, &m)

		fmt.Print(m["stream"])
	}

	return nil
}
