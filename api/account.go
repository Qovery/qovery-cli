package api

import (
	"log"
	"net/http"
)

type User struct {
	ObjectType string `json:"object_type"`
	Id         string `json:"id"`
}

func GetAccount() User {
	var u User
	if err := NewRequest(http.MethodGet, "/account").Do(&u); err != nil {
		log.Fatal(errorUnknownError)
	}
	return u
}
