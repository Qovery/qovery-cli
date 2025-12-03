package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	dnsProvider            string
	dnsDomain              string
	cloudflareEmail        string
	cloudflareToken        string
	cloudflareProxied      bool
	qoveryApiUrl           string
	route53AccessKeyId     string
	route53SecretAccessKey string
	route53Region          string
	route53HostedZoneId    string
)

var adminClusterUpdateDnsProviderCmd = &cobra.Command{
	Use:   "update-dns-provider",
	Short: "Update cluster DNS provider credentials and domain. Cluster and all apps need to be re-deployed after",
	Long: `Update the DNS provider configuration for a cluster. This allows you to switch between or reconfigure
DNS providers (Cloudflare, Qovery, or Route53). After updating, the cluster and all applications must be re-deployed.

Examples:
  # Update to Cloudflare
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider cloudflare --cloudflare-email user@example.com --cloudflare-token <token>

  # Update to Route53
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider route53 --route53-access-key-id <key> --route53-secret-access-key <secret> \
    --route53-region us-east-1 [--route53-hosted-zone-id <zone-id>]

  # Update to Qovery DNS
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider qovery --qovery-api-url https://dns.qovery.com`,
	Run: func(cmd *cobra.Command, args []string) {
		updateClusterDnsProvider()
	},
}

func init() {
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&clusterId, "cluster-id", "", "The cluster id to target (required)")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&dnsDomain, "domain", "", "The domain for the cluster (required)")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&dnsProvider, "provider", "", "DNS provider: cloudflare, qovery, or route53 (required)")

	// Cloudflare flags
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&cloudflareEmail, "cloudflare-email", "", "Cloudflare email")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&cloudflareToken, "cloudflare-token", "", "Cloudflare API token")
	adminClusterUpdateDnsProviderCmd.Flags().BoolVar(&cloudflareProxied, "cloudflare-proxied", false, "Enable Cloudflare proxy")

	// Qovery flags
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&qoveryApiUrl, "qovery-api-url", "", "Qovery DNS API URL")

	// Route53 flags
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&route53AccessKeyId, "route53-access-key-id", "", "AWS Access Key ID for Route53")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&route53SecretAccessKey, "route53-secret-access-key", "", "AWS Secret Access Key for Route53")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&route53Region, "route53-region", "", "AWS Region for Route53")
	adminClusterUpdateDnsProviderCmd.Flags().StringVar(&route53HostedZoneId, "route53-hosted-zone-id", "", "AWS Route53 Hosted Zone ID (optional)")

	adminClusterUpdateDnsProviderCmd.MarkFlagRequired("cluster-id")
	adminClusterUpdateDnsProviderCmd.MarkFlagRequired("domain")
	adminClusterUpdateDnsProviderCmd.MarkFlagRequired("provider")

	adminClusterCmd.AddCommand(adminClusterUpdateDnsProviderCmd)
}

func updateClusterDnsProvider() {
	if clusterId == "" {
		utils.PrintlnError(nil)
		utils.PrintlnInfo("cluster-id is required")
		os.Exit(1)
	}

	if dnsDomain == "" {
		utils.PrintlnError(nil)
		utils.PrintlnInfo("domain is required")
		os.Exit(1)
	}

	if dnsProvider == "" {
		utils.PrintlnError(nil)
		utils.PrintlnInfo("provider is required (cloudflare, qovery, or route53)")
		os.Exit(1)
	}

	// Validate provider-specific flags
	switch dnsProvider {
	case "cloudflare":
		if cloudflareEmail == "" || cloudflareToken == "" {
			utils.PrintlnError(nil)
			utils.PrintlnInfo("--cloudflare-email and --cloudflare-token are required for Cloudflare provider")
			os.Exit(1)
		}
	case "qovery":
		if qoveryApiUrl == "" {
			utils.PrintlnError(nil)
			utils.PrintlnInfo("--qovery-api-url is required for Qovery provider")
			os.Exit(1)
		}
	case "route53":
		if route53AccessKeyId == "" || route53SecretAccessKey == "" || route53Region == "" {
			utils.PrintlnError(nil)
			utils.PrintlnInfo("--route53-access-key-id, --route53-secret-access-key, and --route53-region are required for Route53 provider")
			os.Exit(1)
		}
	default:
		utils.PrintlnError(nil)
		utils.PrintlnInfo("Invalid provider. Must be cloudflare, qovery, or route53")
		os.Exit(1)
	}

	err := pkg.UpdateClusterDnsProvider(
		clusterId,
		dnsDomain,
		dnsProvider,
		cloudflareEmail,
		cloudflareToken,
		cloudflareProxied,
		qoveryApiUrl,
		route53AccessKeyId,
		route53SecretAccessKey,
		route53Region,
		route53HostedZoneId,
	)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	utils.PrintlnInfo("DNS provider updated successfully. Please redeploy the cluster and all applications.")
}
