package util

import (
	"gopkg.in/src-d/go-git.v4"
	"strings"
)

func CurrentBranchName() string {
	repo, err := git.PlainOpen(".git")
	if err != nil {
		return ""
	}

	h, err := repo.Head()
	if err != nil {
		return ""
	}

	branchName := h.Name().String()

	if branchName == "HEAD" {
		return ""
	}

	sBranchName := strings.Split(branchName, "/")
	return sBranchName[2]
}
