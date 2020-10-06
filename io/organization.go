package io

type Organizations struct {
	Results []Organization `json:"results"`
}

type Organization struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Name       string `json:"name"`
}
