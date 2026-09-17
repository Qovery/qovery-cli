<p align="center">
  <img alt="Qovery Logo" src="https://www.qovery.com/logos/qovery-logo-black.svg" />
</p>

[Qovery](https://www.qovery.com/) helps tech companies to accelerate and scale application development cycle with zero infrastructure management investment.

This repository is the code source of the Qovery CLI.

See the [Qovery documentation](https://docs.qovery.com) to get started with Qovery.

See the [Qovery CLI documentation](https://www.qovery.com/docs/cli/overview) to get started with the CLI and explore its commands.

## Installation

Choose the installation method for your platform.

### Linux

Install the latest version on any Linux distribution:

```sh
curl -s https://get.qovery.com | bash
```

For security-sensitive environments, install from a [pinned GitHub release](https://github.com/Qovery/qovery-cli/releases) and verify the downloaded archive against that release's `checksums.txt`. The convenience script does not perform an integrity check.

### macOS

Install with Homebrew:

```sh
brew tap Qovery/qovery-cli
brew install qovery-cli
```

Alternatively, use the installer script:

```sh
curl -s https://get.qovery.com | bash
```

For a pinned, checksum-verified installation, use a [GitHub release](https://github.com/Qovery/qovery-cli/releases).

### Windows

Install with [Scoop](https://scoop.sh/):

```powershell
scoop bucket add qovery https://github.com/Qovery/scoop-qovery-cli
scoop install qovery-cli
```

You can also download a release archive from [GitHub Releases](https://github.com/Qovery/qovery-cli/releases) and add the extracted `qovery` executable to your `PATH`.

### Arch Linux

The CLI is available through the AUR:

```sh
yay qovery-cli
```

### Docker

Run the CLI without installing it locally:

```sh
docker run ghcr.io/qovery/qovery-cli:latest help
```

Replace `latest` with a specific version when you need reproducible builds.

### Verify the installation

```sh
qovery version
```

## Authentication

You can use `qovery auth` to authenticate with the CLI or use `Q_CLI_ACCESS_TOKEN` (or `QOVERY_CLI_ACCESS_TOKEN`) environment variable to set your API token.

# Update deps

```sh
go get -u github.com/qovery/qovery-client-go
go build
go fmt .
```
