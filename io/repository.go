package io

type Repositories struct {
	Results []Repository `json:"results"`
}

type Repository struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	BranchId   string `json:"branch_id"`
	CommitId   string `json:"commit_id"`
}
