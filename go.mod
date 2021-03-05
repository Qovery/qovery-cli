module github.com/Qovery/qovery-cli

go 1.16

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20200531234253-77e06fda0c94

replace github.com/Qovery/qovery-cli/cmd => ./cmd

replace github.com/Qovery/qovery-cli/io => ./io

require (
	github.com/Microsoft/hcsshim v0.8.15 // indirect
	github.com/Qovery/qovery-cli/cmd v0.0.0-00010101000000-000000000000
	github.com/Qovery/qovery-cli/io v0.0.0-00010101000000-000000000000 // indirect
	github.com/containerd/continuity v0.0.0-20210208174643-50096c924a4e // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/mholt/archiver/v3 v3.5.0 // indirect
	github.com/sirupsen/logrus v1.8.0 // indirect
	github.com/spf13/cobra v1.1.3 // indirect
	github.com/xeonx/timeago v1.0.0-rc4 // indirect
	golang.org/x/sys v0.0.0-20210305215415-5cdee2b1b5a0 // indirect
)
