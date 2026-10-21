package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	roleArn                    string
	adminLoadAwsCredentialsCmd = &cobra.Command{
		Use:   "load-aws-credentials",
		Short: "Load aws credentials from a role ARN",
		Long: `This command is used to load aws credentials
The role is assumed with the external id "<organization id>:<cluster id>", the one its trust policy can require.

> Examples
----------
* Load AWS credentials from a role ARN arn:aws:iam::123456789012:role/qovery-user-role-xxx of an organization
qovery admin load-aws-credentials --role-arn arn:aws:iam::123456789012:role/qovery-user-role-xxx --organization-id 12345678-1234-1234-1234-123456789012

* Load AWS credentials from a role ARN on behalf of one of the organization clusters
qovery admin load-aws-credentials --role-arn arn:aws:iam::123456789012:role/qovery-user-role-xxx --organization-id 12345678-1234-1234-1234-123456789012 --cluster-id 87654321-4321-4321-4321-210987654321

`,
		Run: func(cmd *cobra.Command, args []string) {
			err := pkg.LoadAwsCredentials(roleArn, organizationId, clusterId)
			utils.CheckError(err)
		},
	}
)

func init() {
	adminLoadAwsCredentialsCmd.Flags().StringVarP(&roleArn, "role-arn", "r", "", "ARN of the AWS IAM role to assume")
	adminLoadAwsCredentialsCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "ID of the organization the role belongs to")
	adminLoadAwsCredentialsCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "ID of the cluster to assume the role for (optional)")
	_ = adminLoadAwsCredentialsCmd.MarkFlagRequired("organization-id")
	adminCmd.AddCommand(adminLoadAwsCredentialsCmd)
}
