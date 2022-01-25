module github.com/qovery/qovery-cli

go 1.16

replace (
	github.com/qovery/qovery-cli/cmd => ./cmd
	github.com/qovery/qovery-cli/pkg => ./pkg
	github.com/qovery/qovery-cli/utils => ./utils
)

require (
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/qovery/qovery-cli/cmd v0.0.0-00010101000000-000000000000
)
