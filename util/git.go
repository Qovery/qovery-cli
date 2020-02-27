package util

import (
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"sort"
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

func ListRemoteURLs() []string {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return []string{}
	}

	c, err := repo.Config()
	if err != nil {
		return []string{}
	}

	var urls []string
	for _, v := range c.Remotes {
		for _, url := range v.URLs {
			if strings.HasPrefix(url, "git@github.com") {
				url = "https://github.com/" + strings.Split(url, ":")[1]
			} // TODO same for gitlab and bitbucket

			urls = append(urls, url)
		}
	}

	return urls
}

func ListCommits(nLast int) []*object.Commit {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return []*object.Commit{}
	}

	options := git.LogOptions{}
	c, err := repo.Log(&options)
	if err != nil {
		return []*object.Commit{}
	}

	var commits []*object.Commit

	_ = c.ForEach(func(commit *object.Commit) error {
		commits = append(commits, commit)
		return nil
	})

	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Committer.When.Unix() > commits[j].Committer.When.Unix()
	})

	var finalCommits []*object.Commit
	for k, commit := range commits {
		if k == nLast {
			break
		}

		finalCommits = append(finalCommits, commit)
	}

	return finalCommits
}
