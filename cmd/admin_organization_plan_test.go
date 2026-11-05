package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAdminOrganizationPlanNormalizesAcceptedValues(t *testing.T) {
	for _, plan := range []string{"ENTERPRISE_2025", "enterprise_2025", "  Enterprise_2025  "} {
		parsed, err := parseAdminOrganizationPlan(plan)
		if err != nil {
			t.Fatalf("expected %q to be accepted, got %v", plan, err)
		}
		if parsed != "ENTERPRISE_2025" {
			t.Fatalf("expected ENTERPRISE_2025, got %q", parsed)
		}
	}
}

func TestParseAdminOrganizationPlanRejectsUnknownValues(t *testing.T) {
	// TEAM_YEARLY exists in the public API client but not in the Plan enum of Core
	for _, plan := range []string{"", "   ", "entreprise_2025", "TEAM_YEARLY"} {
		_, err := parseAdminOrganizationPlan(plan)
		if err == nil {
			t.Fatalf("expected %q to be rejected", plan)
		}
		if !strings.Contains(err.Error(), "ENTERPRISE_2025") {
			t.Fatalf("expected the accepted plans in the error, got %v", err)
		}
	}
}

func TestRequestAdminOrganizationPlanReadsThePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/organization/org-1/plan" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"organization_id":"org-1","name":"Acme","plan":"FREE","billing_external_id":null}`)
	}))
	defer server.Close()

	body, err := requestAdminOrganizationPlan(http.MethodGet, server.URL+"/", "Bearer token", server.Client(), "org-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"plan":"FREE"`) {
		t.Fatalf("unexpected body %s", body)
	}
}

func TestRequestAdminOrganizationPlanSendsThePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/organization/org-1/plan" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != `{"plan":"ENTERPRISE_2025"}` {
			t.Errorf("unexpected body %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"organization_id":"org-1","plan":"ENTERPRISE_2025","previous_plan":"FREE","warning":"not changed"}`)
	}))
	defer server.Close()

	_, err := requestAdminOrganizationPlan(
		http.MethodPut,
		server.URL,
		"Bearer token",
		server.Client(),
		"org-1",
		[]byte(`{"plan":"ENTERPRISE_2025"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestAdminOrganizationPlanSurfacesErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(writer, "organization not found")
	}))
	defer server.Close()

	_, err := requestAdminOrganizationPlan(http.MethodGet, server.URL, "Bearer token", server.Client(), "org-1", nil)
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "organization not found") {
		t.Fatalf("expected a 404 error carrying the response body, got %v", err)
	}
}
