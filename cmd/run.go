package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path/filepath"
	"qovery.go/io"
)

import iio "io"

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Equivalent to 'docker build' and 'docker run' but with Qovery magic sauce",
	Long: `RUN performs 'docker build' and 'docker run' actions and set Qovery properties to target the right environment . For example:

	qovery run`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true)
		qoveryYML, _ := io.CurrentQoveryYML()

		dockerClient, _ := client.NewClientWithOpts()
		disableBuildCache, _ := cmd.Flags().GetBool("disableBuildCache")
		_, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})

		if err != nil {
			io.PrintError("Run Docker or install it on your system")
			os.Exit(1)
		}

		project := io.GetProjectByName(ProjectName, OrganizationName)
		if project.Id == "" {
			io.PrintError("The project does not exist. Are you well authenticated with the right user? Do 'qovery auth' to be sure")
			os.Exit(1)
		}

		environment := io.GetEnvironmentByName(project.Id, BranchName)
		applications := io.ListApplicationsRaw(project.Id, environment.Id)

		if applications["results"] != nil {
			fmt.Println("Run in progress...")

			results := applications["results"].([]interface{})
			for _, application := range results {
				applicationConfigurationMap := application.(map[string]interface{})
				if applicationConfigurationMap["name"] == ApplicationName {

					var environmentVariables []string
					buildArgs := make(map[string]*string)

					evs := ListEnvironmentVariables(OrganizationName, ProjectName, BranchName, ApplicationName)

					for i := range evs {
						ev := evs[i]
						if ev.KeyValue != "" {
							environmentVariables = append(environmentVariables, ev.KeyValue)
							buildArgs[ev.Key] = &ev.Value
						}
					}

					envs := make(map[string]string)
					for _, ev := range evs {
						envs[ev.Key] = ev.Value
					}

					for k, v := range io.GetDotEnvs(envs) {
						environmentVariables = append(environmentVariables, k+"="+v)
						buildArgs[k] = &v
					}

					image := buildContainer(dockerClient, qoveryYML.Application.DockerfilePath(), buildArgs, disableBuildCache)
					runContainer(dockerClient, image, environmentVariables)

					break
				}
			}
		} else {
			fmt.Println("Please Commit and Push your project at least one time. We need to set up the remote environment first!")
		}
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&ConfigurationDirectoryRoot, "configuration-directory-root", "c", ".", "Your configuration directory root path")
	runCmd.Flags().BoolP("disableBuildCache", "n", true, "Disable Docker build cache in order to reduce server side build issue")

	RootCmd.AddCommand(runCmd)
}

func buildContainer(client *client.Client, dockerfilePath string, buildArgs map[string]*string, disableBuildCache bool) *types.ImageSummary {
	currentDir, _ := os.Getwd()
	excludes, err := ReadDockerignore(currentDir)

	if err != nil {
		println("Could not load .dockerignore content")
	}

	excludes = TrimBuildFilesFromExcludes(excludes, "Dockerfile")
	buildCtx, err := archive.TarWithOptions(currentDir, &archive.TarOptions{
		ExcludePatterns: excludes,
		ChownOpts:       &idtools.Identity{UID: 0, GID: 0},
	})

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	r, err := client.ImageBuild(context.Background(), buildCtx, types.ImageBuildOptions{
		Dockerfile: dockerfilePath,
		BuildArgs:  buildArgs,
		NoCache: disableBuildCache,
	})

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer r.Body.Close()

	_ = writeToLog(r.Body)

	images, err := client.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	// last created image // TODO change this, it is not good
	image := images[0]

	return &image
}

func runContainer(client *client.Client, image *types.ImageSummary, environmentVariables []string) {
	config := &container.Config{Image: image.ID, Env: environmentVariables}

	hostConfig := &container.HostConfig{}

	exposePorts := io.ExposePortsFromCurrentDockerfile()

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

		_, _ = iio.Copy(os.Stdout, out)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			// sig is a ^C, handle it
			_ = client.ContainerStop(context.Background(), c.ID, nil)
		}
	}()

	statusCh, errCh := client.ContainerWait(context.Background(), c.ID, container.WaitConditionNextExit)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}
}

func writeToLog(reader iio.ReadCloser) error {
	defer reader.Close()
	rd := bufio.NewReader(reader)

	for {
		n, _, err := rd.ReadLine()
		if err != nil && err == iio.EOF {
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

//func getApplicationConfigByName(projectId string, branchName string, appName string) map[string]interface{} {
//	return filterApplicationsByName(io.ListApplicationsRaw(projectId, branchName), appName)
//}

func filterApplicationsByName(applications map[string]interface{}, appName string) map[string]interface{} {
	if val, ok := applications["results"]; ok {
		results := val.([]interface{})
		for _, application := range results {
			a := application.(map[string]interface{})
			if name, found := a["name"]; found && name == appName {
				return a
			}
		}
	}
	return nil
}

func ReadDockerignore(contextDir string) ([]string, error) {
	var excludes []string

	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	return dockerignore.ReadAll(f)
}

// TrimBuildFilesFromExcludes removes the named Dockerfile and .dockerignore from
// the list of excluded files. The daemon will remove them from the final context
// but they must be in available in the context when passed to the API.
func TrimBuildFilesFromExcludes(excludes []string, dockerfile string) []string {
	if keep, _ := fileutils.Matches(".dockerignore", excludes); keep {
		excludes = append(excludes, "!.dockerignore")
	}
	if keep, _ := fileutils.Matches(dockerfile, excludes); keep {
		excludes = append(excludes, "!"+dockerfile)
	}
	return excludes
}
