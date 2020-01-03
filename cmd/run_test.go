package cmd

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFilterApplicationByName(t *testing.T) {
	var apps map[string]interface{}
	if err := json.Unmarshal([]byte(testValueJsonApps), &apps); err != nil {
		t.Errorf("fail to unmarshal test value: %s", err.Error())
	}
	assert.NotEmpty(t, apps, "it should not be empty")

	result := filterApplicationsByName(apps, "MYAPPNAME")
	assert.NotNil(t, result, "it should not be nil")
	assert.Equal(t, "MYAPPNAME", result["name"])
}

const (
	testValueJsonApps = `{
  "results": [
    {
      "brokers": [
        {
          "category": "UNKNOWN",
          "connection_uri": "string",
          "created_at": "2020-01-03T11:14:01.419Z",
          "disk_size_in_mb": 0,
          "environment": {
            "branch_id": "string",
            "brokers": [
              null
            ],
            "commit_id": "string",
            "created_at": "2020-01-03T11:14:01.419Z",
            "databases": [
              null
            ],
            "id": "string",
            "name": "MYAPPNAME",
            "normalized_name": "string",
            "object_type": "string",
            "project_id": "string",
            "repository": {
              "created_at": "2020-01-03T11:14:01.420Z",
              "external_id": "string",
              "id": "string",
              "is_access_token_set": true,
              "login": "string",
              "name": "string",
              "object_type": "string",
              "project": {
                "cloud_provider_region": {
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "full_name": "string",
                  "id": "string",
                  "name": "string",
                  "object_type": "string",
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "created_at": "2020-01-03T11:14:01.421Z",
                "id": "string",
                "members": [
                  {
                    "company": "string",
                    "country": "string",
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "first_name": "string",
                    "id": "string",
                    "job_position": "string",
                    "last_name": "string",
                    "number_of_employees": 0,
                    "object_type": "string",
                    "phone_number": "string",
                    "profile_complete": true,
                    "properties": {
                      "email": "string",
                      "email_verified": true,
                      "name": "string",
                      "nickname": "string",
                      "picture": "string",
                      "sub": "string"
                    },
                    "temporary_account": true,
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "name": "string",
                "object_type": "string",
                "owner": {
                  "company": "string",
                  "country": "string",
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "first_name": "string",
                  "id": "string",
                  "job_position": "string",
                  "last_name": "string",
                  "number_of_employees": 0,
                  "object_type": "string",
                  "phone_number": "string",
                  "profile_complete": true,
                  "properties": {
                    "email": "string",
                    "email_verified": true,
                    "name": "string",
                    "nickname": "string",
                    "picture": "string",
                    "sub": "string"
                  },
                  "temporary_account": true,
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "updated_at": "2020-01-03T11:14:01.421Z"
              },
              "updated_at": "2020-01-03T11:14:01.421Z",
              "url": "string"
            },
            "routers": [
              {
                "connection_uri": "string",
                "created_at": "2020-01-03T11:14:01.421Z",
                "custom_fqdn": "string",
                "fqdn": "string",
                "id": "string",
                "name": "string",
                "object_type": "string",
                "public_port": 0,
                "routes": [
                  {
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "id": "string",
                    "object_type": "string",
                    "path": "string",
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "updated_at": "2020-01-03T11:14:01.421Z"
              }
            ],
            "services": [
              null
            ],
            "status": "LIVE",
            "storage": [
              null
            ],
            "total_brokers": 0,
            "total_databases": 0,
            "total_services": 0,
            "total_storage": 0,
            "total_unknown": 0,
            "updated_at": "2020-01-03T11:14:01.421Z"
          },
          "fqdn": "string",
          "id": "string",
          "name": "string",
          "object_type": "string",
          "password": "string",
          "port": 0,
          "status": "LIVE",
          "type": "string",
          "updated_at": "2020-01-03T11:14:01.421Z",
          "username": "string",
          "version": "string"
        }
      ],
      "connection_uri": "string",
      "created_at": "2020-01-03T11:14:01.421Z",
      "databases": [
        {
          "category": "UNKNOWN",
          "connection_uri": "string",
          "created_at": "2020-01-03T11:14:01.421Z",
          "disk_size_in_mb": 0,
          "environment": {
            "branch_id": "string",
            "brokers": [
              null
            ],
            "commit_id": "string",
            "created_at": "2020-01-03T11:14:01.421Z",
            "databases": [
              null
            ],
            "id": "string",
            "name": "string",
            "normalized_name": "string",
            "object_type": "string",
            "project_id": "string",
            "repository": {
              "created_at": "2020-01-03T11:14:01.421Z",
              "external_id": "string",
              "id": "string",
              "is_access_token_set": true,
              "login": "string",
              "name": "string",
              "object_type": "string",
              "project": {
                "cloud_provider_region": {
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "full_name": "string",
                  "id": "string",
                  "name": "string",
                  "object_type": "string",
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "created_at": "2020-01-03T11:14:01.421Z",
                "id": "string",
                "members": [
                  {
                    "company": "string",
                    "country": "string",
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "first_name": "string",
                    "id": "string",
                    "job_position": "string",
                    "last_name": "string",
                    "number_of_employees": 0,
                    "object_type": "string",
                    "phone_number": "string",
                    "profile_complete": true,
                    "properties": {
                      "email": "string",
                      "email_verified": true,
                      "name": "string",
                      "nickname": "string",
                      "picture": "string",
                      "sub": "string"
                    },
                    "temporary_account": true,
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "name": "string",
                "object_type": "string",
                "owner": {
                  "company": "string",
                  "country": "string",
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "first_name": "string",
                  "id": "string",
                  "job_position": "string",
                  "last_name": "string",
                  "number_of_employees": 0,
                  "object_type": "string",
                  "phone_number": "string",
                  "profile_complete": true,
                  "properties": {
                    "email": "string",
                    "email_verified": true,
                    "name": "string",
                    "nickname": "string",
                    "picture": "string",
                    "sub": "string"
                  },
                  "temporary_account": true,
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "updated_at": "2020-01-03T11:14:01.421Z"
              },
              "updated_at": "2020-01-03T11:14:01.421Z",
              "url": "string"
            },
            "routers": [
              {
                "connection_uri": "string",
                "created_at": "2020-01-03T11:14:01.421Z",
                "custom_fqdn": "string",
                "fqdn": "string",
                "id": "string",
                "name": "string",
                "object_type": "string",
                "public_port": 0,
                "routes": [
                  {
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "id": "string",
                    "object_type": "string",
                    "path": "string",
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "updated_at": "2020-01-03T11:14:01.421Z"
              }
            ],
            "services": [
              null
            ],
            "status": "LIVE",
            "storage": [
              null
            ],
            "total_brokers": 0,
            "total_databases": 0,
            "total_services": 0,
            "total_storage": 0,
            "total_unknown": 0,
            "updated_at": "2020-01-03T11:14:01.421Z"
          },
          "fqdn": "string",
          "id": "string",
          "name": "string",
          "object_type": "string",
          "password": "string",
          "port": 0,
          "status": "LIVE",
          "type": "string",
          "updated_at": "2020-01-03T11:14:01.421Z",
          "username": "string",
          "version": "string"
        }
      ],
      "dockerfile_content": "string",
      "environment": {
        "branch_id": "string",
        "brokers": [
          null
        ],
        "commit_id": "string",
        "created_at": "2020-01-03T11:14:01.421Z",
        "databases": [
          null
        ],
        "id": "string",
        "name": "string",
        "normalized_name": "string",
        "object_type": "string",
        "project_id": "string",
        "repository": {
          "created_at": "2020-01-03T11:14:01.421Z",
          "external_id": "string",
          "id": "string",
          "is_access_token_set": true,
          "login": "string",
          "name": "string",
          "object_type": "string",
          "project": {
            "cloud_provider_region": {
              "created_at": "2020-01-03T11:14:01.421Z",
              "full_name": "string",
              "id": "string",
              "name": "string",
              "object_type": "string",
              "updated_at": "2020-01-03T11:14:01.421Z"
            },
            "created_at": "2020-01-03T11:14:01.421Z",
            "id": "string",
            "members": [
              {
                "company": "string",
                "country": "string",
                "created_at": "2020-01-03T11:14:01.421Z",
                "first_name": "string",
                "id": "string",
                "job_position": "string",
                "last_name": "string",
                "number_of_employees": 0,
                "object_type": "string",
                "phone_number": "string",
                "profile_complete": true,
                "properties": {
                  "email": "string",
                  "email_verified": true,
                  "name": "string",
                  "nickname": "string",
                  "picture": "string",
                  "sub": "string"
                },
                "temporary_account": true,
                "updated_at": "2020-01-03T11:14:01.421Z"
              }
            ],
            "name": "string",
            "object_type": "string",
            "owner": {
              "company": "string",
              "country": "string",
              "created_at": "2020-01-03T11:14:01.421Z",
              "first_name": "string",
              "id": "string",
              "job_position": "string",
              "last_name": "string",
              "number_of_employees": 0,
              "object_type": "string",
              "phone_number": "string",
              "profile_complete": true,
              "properties": {
                "email": "string",
                "email_verified": true,
                "name": "string",
                "nickname": "string",
                "picture": "string",
                "sub": "string"
              },
              "temporary_account": true,
              "updated_at": "2020-01-03T11:14:01.421Z"
            },
            "updated_at": "2020-01-03T11:14:01.421Z"
          },
          "updated_at": "2020-01-03T11:14:01.421Z",
          "url": "string"
        },
        "routers": [
          {
            "connection_uri": "string",
            "created_at": "2020-01-03T11:14:01.421Z",
            "custom_fqdn": "string",
            "fqdn": "string",
            "id": "string",
            "name": "string",
            "object_type": "string",
            "public_port": 0,
            "routes": [
              {
                "created_at": "2020-01-03T11:14:01.421Z",
                "id": "string",
                "object_type": "string",
                "path": "string",
                "updated_at": "2020-01-03T11:14:01.421Z"
              }
            ],
            "updated_at": "2020-01-03T11:14:01.421Z"
          }
        ],
        "services": [
          null
        ],
        "status": "LIVE",
        "storage": [
          null
        ],
        "total_brokers": 0,
        "total_databases": 0,
        "total_services": 0,
        "total_storage": 0,
        "total_unknown": 0,
        "updated_at": "2020-01-03T11:14:01.421Z"
      },
      "fqdn": "string",
      "id": "string",
      "name": "MYAPPNAME",
      "object_type": "string",
      "private_port": 0,
      "public_port": 0,
      "status": "LIVE",
      "storage": [
        {
          "category": "UNKNOWN",
          "connection_uri": "string",
          "created_at": "2020-01-03T11:14:01.421Z",
          "disk_size_in_mb": 0,
          "environment": {
            "branch_id": "string",
            "brokers": [
              null
            ],
            "commit_id": "string",
            "created_at": "2020-01-03T11:14:01.421Z",
            "databases": [
              null
            ],
            "id": "string",
            "name": "string",
            "normalized_name": "string",
            "object_type": "string",
            "project_id": "string",
            "repository": {
              "created_at": "2020-01-03T11:14:01.421Z",
              "external_id": "string",
              "id": "string",
              "is_access_token_set": true,
              "login": "string",
              "name": "string",
              "object_type": "string",
              "project": {
                "cloud_provider_region": {
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "full_name": "string",
                  "id": "string",
                  "name": "string",
                  "object_type": "string",
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "created_at": "2020-01-03T11:14:01.421Z",
                "id": "string",
                "members": [
                  {
                    "company": "string",
                    "country": "string",
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "first_name": "string",
                    "id": "string",
                    "job_position": "string",
                    "last_name": "string",
                    "number_of_employees": 0,
                    "object_type": "string",
                    "phone_number": "string",
                    "profile_complete": true,
                    "properties": {
                      "email": "string",
                      "email_verified": true,
                      "name": "string",
                      "nickname": "string",
                      "picture": "string",
                      "sub": "string"
                    },
                    "temporary_account": true,
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "name": "string",
                "object_type": "string",
                "owner": {
                  "company": "string",
                  "country": "string",
                  "created_at": "2020-01-03T11:14:01.421Z",
                  "first_name": "string",
                  "id": "string",
                  "job_position": "string",
                  "last_name": "string",
                  "number_of_employees": 0,
                  "object_type": "string",
                  "phone_number": "string",
                  "profile_complete": true,
                  "properties": {
                    "email": "string",
                    "email_verified": true,
                    "name": "string",
                    "nickname": "string",
                    "picture": "string",
                    "sub": "string"
                  },
                  "temporary_account": true,
                  "updated_at": "2020-01-03T11:14:01.421Z"
                },
                "updated_at": "2020-01-03T11:14:01.421Z"
              },
              "updated_at": "2020-01-03T11:14:01.421Z",
              "url": "string"
            },
            "routers": [
              {
                "connection_uri": "string",
                "created_at": "2020-01-03T11:14:01.421Z",
                "custom_fqdn": "string",
                "fqdn": "string",
                "id": "string",
                "name": "string",
                "object_type": "string",
                "public_port": 0,
                "routes": [
                  {
                    "created_at": "2020-01-03T11:14:01.421Z",
                    "id": "string",
                    "object_type": "string",
                    "path": "string",
                    "updated_at": "2020-01-03T11:14:01.421Z"
                  }
                ],
                "updated_at": "2020-01-03T11:14:01.421Z"
              }
            ],
            "services": [
              null
            ],
            "status": "LIVE",
            "storage": [
              null
            ],
            "total_brokers": 0,
            "total_databases": 0,
            "total_services": 0,
            "total_storage": 0,
            "total_unknown": 0,
            "updated_at": "2020-01-03T11:14:01.421Z"
          },
          "fqdn": "string",
          "id": "string",
          "name": "string",
          "object_type": "string",
          "password": "string",
          "port": 0,
          "status": "LIVE",
          "type": "string",
          "updated_at": "2020-01-03T11:14:01.421Z",
          "username": "string",
          "version": "string"
        }
      ],
      "total_brokers": 0,
      "total_databases": 0,
      "total_services": 0,
      "total_storage": 0,
      "total_unknown": 0,
      "updated_at": "2020-01-03T11:14:01.421Z"
    }
  ]
}`
)
