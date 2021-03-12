package io

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Organizations struct {
	Results []Organization `json:"results"`
}

type Organization struct {
	ObjectType         string `json:"object_type"`
	Id                 string `json:"id"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	Name               string `json:"name"`
	DisplayName        string `json:"display_name"`
	IsRealOrganization bool   `json:"is_real_organization"`
}

func ListOrganizations() Organizations {
	organizations := Organizations{}
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/organization", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return organizations
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &organizations)

	filteredOrganizations := Organizations{}

	for _, org := range organizations.Results {
		if org.IsRealOrganization {
			filteredOrganizations.Results = append(filteredOrganizations.Results, org)
		}
	}

	return filteredOrganizations
}

func GetPrivateOrganization() Organization {
	p := Organization{}
	CheckAuthenticationOrQuitWithMessage()

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/organization/private", nil)
	req.Header.Set(headerAuthorization, headerValueBearer+GetAuthorizationToken())

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return p
	}

	err = CheckHTTPResponse(resp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	_ = json.Unmarshal(body, &p)

	return p
}

func AddUserToOrganization(organizationName string, userInfo string)  {
	_, adminToken := GetTokens()
	json, _ := json.Marshal(userInfo)
	payload := bytes.NewBuffer(json)

	request, err := http.NewRequest("POST", DefaultRootUrl + "/admin/organization/" + organizationName, payload)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer " + strings.TrimSpace(adminToken))
	request.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != 200 {
		log.Printf("Could not add user to organization.")
	} else {
		log.Printf("User added to " + organizationName + ".")
	}
}
