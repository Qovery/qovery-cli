package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type User struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
}

func GetAccount() User {
	req, _ := http.NewRequest(http.MethodGet, RootURL+"/account", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	u := User{}

	if err != nil {
		return u
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &u)

	return u
}
