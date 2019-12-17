package util

import (
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"strings"
)

func CurrentBranchName() string {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return ""
	}

	r, err := repo.Head()
	if err != nil {
		return ""
	}

	branchName := r.Name().String()

	if branchName == "HEAD" {
		return ""
	}

	sBranchName := strings.Split(branchName, "/")
	return sBranchName[2]
}

func Checkout(branch string) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		fmt.Println(err)
		return
	}

	w, err := repo.Worktree()
	if err != nil {
		fmt.Println(err)
		return
	}

	_ = w.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))})
}
