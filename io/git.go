package io

import (
	"encoding/json"
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GitAccessStatus struct {
	HasAccess       bool   `json:"has_access"`
	Message         string `json:"message"`
	GitURL          string `json:"git_url"`
	SanitizedGitURL string `json:"sanitized_git_url"`
}

func GitCheck(gitURL string) GitAccessStatus {
	gas := GitAccessStatus{}

	if gitURL == "" {
		return gas
	}

	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/git/access/check?url="+gitURL, nil)

	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return gas
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &gas)

	return gas
}

func getGitRepository(path string) (*git.Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		if path == "" {
			return nil, err
		}

		return getGitRepository(GetAbsoluteParentPath(path))
	}

	return repo, nil
}

func CurrentBranchName() string {
	pwd, _ := os.Getwd()
	return CurrentBranchNameFromPath(filepath.Join(pwd, "."))
}

func CurrentBranchNameFromPath(path string) string {
	repo, err := getGitRepository(path)
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
	return strings.Join(sBranchName[2:], "/")
}

func Checkout(branch string) {
	pwd, _ := os.Getwd()
	CheckoutFromPath(branch, filepath.Join(pwd, "."))
}

func CheckoutFromPath(branch string, path string) {
	repo, err := getGitRepository(path)
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
	pwd, _ := os.Getwd()
	return ListRemoteURLsFromPath(filepath.Join(pwd, "."))
}

func ListRemoteURLsFromPath(path string) []string {
	repo, err := getGitRepository(path)
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
			} else if strings.HasPrefix(url, "git@gitlab.com") {
				url = "https://gitlab.com/" + strings.Split(url, ":")[1]
			} else if strings.HasPrefix(url, "git@bitbucket.com") {
				url = "https://bitbucket.com/" + strings.Split(url, ":")[1]
			}

			urls = append(urls, url)
		}
	}

	return urls
}

func ListCommits(nLast int) []*object.Commit {
	pwd, _ := os.Getwd()
	return ListCommitsFromPath(nLast, filepath.Join(pwd, "."))
}

func ListCommitsFromPath(nLast int, path string) []*object.Commit {
	repo, err := getGitRepository(path)
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
		if isPushedToRemote(repo, commit) {
			commits = append(commits, commit)
		}
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

func isPushedToRemote(repo *git.Repository, commit *object.Commit) bool {
	revision := "origin/" + CurrentBranchName()

	revHash, err := repo.ResolveRevision(plumbing.Revision(revision))
	CheckIfError(err)

	revCommit, err := repo.CommitObject(*revHash)
	CheckIfError(err)

	isPushed, err := commit.IsAncestor(revCommit)
	CheckIfError(err)

	return isPushed
}

func CheckIfError(err error) {
	if err != nil {
		println(err)
		os.Exit(1)
	}
}

func InitializeEmptyGitRepository(folder string) error {
	repository, e := git.PlainInit(folder, false)
	if e != nil {
		return e
	}
	worktree, e := repository.Worktree()
	if e != nil {
		return e
	}
	_, e = worktree.Add(".")
	if e != nil {
		return e
	}
	_, e = worktree.Commit("Initial commit from Qovery", &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "Qovery CLI",
			Email: "hello@qovery.com",
			When:  time.Now(),
		},
	})
	if e != nil {
		return e
	}

	return nil
}
