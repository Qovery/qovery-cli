module github.com/Qovery/qovery-cli/cmd

go 1.16

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20200531234253-77e06fda0c94
replace github.com/Qovery/qovery-cli/io => ../io

require (
	github.com/Microsoft/hcsshim v0.8.15 // indirect
	github.com/Qovery/qovery-cli/io v0.0.0-00010101000000-000000000000
	github.com/containerd/continuity v0.0.0-20210208174643-50096c924a4e // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/docker v20.10.5+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/fatih/color v1.10.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/mholt/archiver/v3 v3.5.0
	github.com/moby/buildkit v0.8.2 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/pkg/browser v0.0.0-20210115035449-ce105d075bb4
	github.com/sirupsen/logrus v1.8.0
	github.com/spf13/cobra v1.1.3
	github.com/xeonx/timeago v1.0.0-rc4
	golang.org/x/sys v0.0.0-20210305215415-5cdee2b1b5a0
	gopkg.in/src-d/go-git.v4 v4.13.1
)
