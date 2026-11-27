package pkg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestDatabaseDetachSendsReasonAndDryRun(t *testing.T) {
	var got databaseDetachRequest
	var path, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"database_id":"db-1","database_name":"orders-db","variables_to_delete":["QOVERY_POSTGRESQL_ZDB1_HOST"],"blockers":[],"dry_run":true}`))
	}))
	defer server.Close()

	report, err := requestDatabaseDetach(server.Client(), server.URL+"/admin/", "Bearer token", "db-1", "ticket 1234", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/admin/database/db-1/detach" {
		t.Errorf("path = %q", path)
	}
	if authorization != "Bearer token" {
		t.Errorf("authorization = %q", authorization)
	}
	if got.Reason != "ticket 1234" || !got.DryRun {
		t.Errorf("request = %+v", got)
	}
	if report.DatabaseName != "orders-db" || len(report.VariablesToDelete) != 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestRequestDatabaseDetachReturnsServerRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"Database cannot be detached: environment is DEPLOYING"}`))
	}))
	defer server.Close()

	_, err := requestDatabaseDetach(server.Client(), server.URL+"/admin", "Bearer token", "db-1", "ticket", false)
	if err == nil {
		t.Fatal("expected an error on 409")
	}
	if !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "DEPLOYING") {
		t.Errorf("error = %v", err)
	}
}

func TestRequestDatabaseDetachRejectsResponsesThatDoNotMatchTheRequest(t *testing.T) {
	cases := map[string]struct {
		body   string
		dryRun bool
	}{
		"empty object":           {`{}`, true},
		"null":                   {`null`, true},
		"other database":         {`{"database_id":"db-2","dry_run":true}`, true},
		"dry run on execute":     {`{"database_id":"db-1","dry_run":true}`, false},
		"execute not detached":   {`{"database_id":"db-1","dry_run":false,"detached":false}`, false},
		"dry run says detached":  {`{"database_id":"db-1","dry_run":true,"detached":true}`, true},
		"detached with blockers": {`{"database_id":"db-1","dry_run":false,"detached":true,"blockers":["x"]}`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(c.body))
			}))
			defer server.Close()

			if _, err := requestDatabaseDetach(server.Client(), server.URL+"/admin", "Bearer token", "db-1", "ticket", c.dryRun); err == nil {
				t.Errorf("expected an error for %s", c.body)
			}
		})
	}
}

func TestFormatDatabaseDetachReportShowsBlockersAndBanner(t *testing.T) {
	out := formatDatabaseDetachReport(&DatabaseDetachReport{
		DatabaseID:        "db-1",
		DatabaseName:      "orders-db",
		VariablesToDelete: []string{"QOVERY_POSTGRESQL_ZDB1_HOST"},
		Blockers:          []string{"variable DATABASE_HOST (ALIAS, CONTAINER c-1) would be deleted with the built-in variables"},
	})
	for _, want := range []string{"orders-db (db-1)", "QOVERY_POSTGRESQL_ZDB1_HOST", "DATABASE_HOST (ALIAS", "will NOT be touched"} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}
