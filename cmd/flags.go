package cmd

//flags used by more than 1 command
var (
	DebugFlag                  bool
	WatchFlag                  bool
	DeploymentOutputFlag       bool
	FollowFlag                 bool
	EnvironmentFlag            bool
	Name                       string
	ApplicationName            string
	ProjectName                string
	OrganizationName           string
	BranchName                 string
	ShowCredentials            bool
	OutputEnvironmentVariables bool
	Tail                       int
	ConfigurationDirectoryRoot string
)
