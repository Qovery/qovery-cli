module github.com/qovery/qovery-cli/cmd

go 1.16

replace (
	github.com/qovery/qovery-cli/pkg => ../pkg
	github.com/qovery/qovery-cli/utils => ../utils
)

require (
	github.com/getsentry/sentry-go v0.11.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/mholt/archiver/v3 v3.5.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/pkg/browser v0.0.0-20210904010418-6d279e18f982
	github.com/pterm/pterm v0.12.30
	github.com/qovery/qovery-cli/pkg v0.0.0-00010101000000-000000000000
	github.com/qovery/qovery-cli/utils v0.0.0-00010101000000-000000000000
	github.com/qovery/qovery-client-go v0.0.0-20210713083701-176aa737a39a
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	golang.org/x/net v0.0.0-20210908191846-a5e095526f91
	golang.org/x/sys v0.0.0-20210909193231-528a39cd75f3
)
