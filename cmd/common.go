package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"strings"
)

func LoadCommandOptions(cmd *cobra.Command, isOrganizationMandatory bool, isProjectMandatory bool, isBranchMandatory bool, isApplicationMandatory bool) {
	var errors []string

	if BranchName == "" {
		BranchName = io.CurrentBranchName()
	}

	qoveryYML, _ := io.CurrentQoveryYML(BranchName)

	if OrganizationName != "" {
		// do not do anything
	} else if OrganizationName == "" && qoveryYML.Application.Organization != "" {
		OrganizationName = qoveryYML.Application.Organization
	} else {
		OrganizationName = "QoveryCommunity"
	}

	if ProjectName == "" {
		ProjectName = qoveryYML.Application.Project
	}

	if ApplicationName == "" {
		ApplicationName = qoveryYML.Application.GetSanitizeName()
	}

	if isOrganizationMandatory && OrganizationName == "" {
		errors = append(errors, "organization (-o)")
	}

	if isProjectMandatory && ProjectName == "" {
		errors = append(errors, "project (-p)")
	}

	if isBranchMandatory && BranchName == "" {
		errors = append(errors, "branch (-b)")
	}

	if isApplicationMandatory && ApplicationName == "" {
		errors = append(errors, "application (-a)")
	}

	if len(errors) == 1 {
		io.PrintError(fmt.Sprintf("%s option is mandatory", errors[0]))
	} else if len(errors) > 1 {
		io.PrintError(fmt.Sprintf("%s options are mandatory", strings.Join(errors, ", ")))
	}

	if len(errors) > 0 {
		_ = cmd.Help()
		os.Exit(1)
	}
}
