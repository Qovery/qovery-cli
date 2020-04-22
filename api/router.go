package api

type Router struct {
	ConnectionURI string         `json:"connection_uri"`
	CustomDomains []CustomDomain `json:"custom_domains"`
}

type CustomDomain struct {
	Domain           string `json:"domain"`
	ValidationDomain string `json:"validation_domain"`
	Status           Status `json:"status"`
}
