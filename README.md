<p align="center">
  <img alt="Qovery Logo" src="https://raw.githubusercontent.com/Qovery/public-resources/master/qovery%20logo%20horizontal%20without%20margin.png" />
</p>

[Qovery](https://www.qovery.com/) helps tech companies to accelerate and scale application development cycle with zero infrastructure management investment.

This repository is the code source of the Qovery CLI.

See our complete documentation [here](https://docs.qovery.com) to get started with Qovery.

## Authentication

You can use `qovery auth` to authenticate with the CLI or use `Q_CLI_ACCESS_TOKEN` (or `QOVERY_CLI_ACCESS_TOKEN`) environment variable to set your API token.

## Versions

You can install the latest version of the CLI:
* On Mac: with brew `brew install qovery-cli`
* On ArchLinux: with `yay qovery-cli`
* On Windows: with scoop `scoop install qovery-cli`
* On Docker: at the address `public.ecr.aws/r3m4q3r9/qovery-cli`
* From binary: https://github.com/Qovery/qovery-cli/releases


# Update deps
go get -u github.com/qovery/qovery-client-go
go build
go fmt .
