package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type User struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
}

func GetAccount() User {
	req, _ := http.NewRequest("GET", RootURL+"/account", nil)
	req.Header.Set("Authorization", "Bearer "+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	CheckHTTPResponse(resp)

	u := User{}

	if err != nil {
		return u
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &u)

	return u
}
