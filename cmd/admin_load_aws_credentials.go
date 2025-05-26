package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	roleArn                    string
	adminLoadAwsCredentialsCmd = &cobra.Command{
		Use:   "load-aws-credentials",
		Short: "Load aws credentials from a role ARN",
		Long: `This command is used to load aws credentials 
> Examples
----------
* Load AWS credentials from a role ARN arn:aws:iam::123456789012:role/qovery
qovery admin load-aws-credentials --role-arn arn:aws:iam::123456789012:role/qovery

`,
		Run: func(cmd *cobra.Command, args []string) {
			pkg.LoadAwsCredentials(roleArn)
		},
	}
)

func init() {
	adminLoadAwsCredentialsCmd.Flags().StringVarP(&roleArn, "role-arn", "r", "", "ARN of the AWS IAM role to assume")
	adminClusterCmd.AddCommand(adminLoadAwsCredentialsCmd)
}
