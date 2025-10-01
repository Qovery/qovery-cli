package enterpriseconnection

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

// EnterpriseConnectionService provides centralized operations for enterprise connections
type EnterpriseConnectionService struct {
	client               *qovery.APIClient
	organizationId       string
	availableRolesByName map[string]string // roleName (lowercase) -> roleId
	customRoleNamesById  map[string]string // roleId -> roleName
	customRoleIdsByName  map[string]string // roleName (lowercase) -> roleId
}

// NewEnterpriseConnectionService creates a new service instance with authentication
func NewEnterpriseConnectionService(organizationName string) (*EnterpriseConnectionService, error) {
	// Get access token and client
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := utils.GetQoveryClient(tokenType, token)

	organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	if err != nil {
		return nil, err
	}

	service := &EnterpriseConnectionService{
		client:         client,
		organizationId: organizationId,
	}

	// Initialize role mappings
	if err := service.initializeRoleMappings(); err != nil {
		return nil, err
	}

	return service, nil
}

// initializeRoleMappings loads and caches role information
func (s *EnterpriseConnectionService) initializeRoleMappings() error {
	// Fetch available roles
	availableRoles, _, err := s.client.OrganizationMainCallsAPI.ListOrganizationAvailableRoles(context.Background(), s.organizationId).Execute()
	if err != nil {
		return err
	}

	s.availableRolesByName = make(map[string]string)
	for _, role := range availableRoles.Results {
		s.availableRolesByName[strings.ToLower(role.Name)] = role.Id
	}

	// Fetch custom roles
	customRoles, _, err := s.client.OrganizationCustomRoleAPI.ListOrganizationCustomRoles(context.Background(), s.organizationId).Execute()
	if err != nil {
		return err
	}

	s.customRoleNamesById = make(map[string]string)
	s.customRoleIdsByName = make(map[string]string)
	for _, role := range customRoles.Results {
		s.customRoleNamesById[*role.Id] = *role.Name
		s.customRoleIdsByName[strings.ToLower(*role.Name)] = *role.Id
	}

	return nil
}

// GetEnterpriseConnection retrieves an enterprise connection by name
func (s *EnterpriseConnectionService) GetEnterpriseConnection(connectionName string) (*qovery.EnterpriseConnectionDto, error) {
	connection, _, err := s.client.OrganizationEnterpriseConnectionAPI.GetOrganizationEnterpriseConnection(
		context.Background(),
		s.organizationId,
		connectionName,
	).Execute()
	return connection, err
}

// UpdateEnterpriseConnection updates an enterprise connection
func (s *EnterpriseConnectionService) UpdateEnterpriseConnection(connectionName string, dto qovery.EnterpriseConnectionDto) (*qovery.EnterpriseConnectionDto, error) {
	connection, _, err := s.client.OrganizationEnterpriseConnectionAPI.UpdateOrganizationEnterpriseConnection(
		context.Background(),
		s.organizationId,
		connectionName,
	).EnterpriseConnectionDto(dto).Execute()
	return connection, err
}

// ResolveRoleDisplayName converts role UUID to display name if applicable
func (s *EnterpriseConnectionService) ResolveRoleDisplayName(roleIdOrName string) string {
	if err := uuid.Validate(roleIdOrName); err == nil {
		// It's a UUID, try to find the display name
		if roleName, exists := s.customRoleNamesById[roleIdOrName]; exists {
			return roleName
		}
	}
	// Not a UUID or UUID not found, return as-is
	return roleIdOrName
}

// ResolveProvidedRoleNameOrCustomRoleId resolves a role name to its ID (handles both regular and custom roles)
func (s *EnterpriseConnectionService) ResolveProvidedRoleNameOrCustomRoleId(roleName string) (string, error) {
	// Check if it's a custom role
	lowerRoleName := strings.ToLower(roleName)

	// Return custom role id if exists
	if value, exists := s.customRoleIdsByName[lowerRoleName]; exists {
		return value, nil
	}

	// Return provided role name
	if _, exists := s.availableRolesByName[lowerRoleName]; exists {
		return lowerRoleName, nil
	}

	return "", fmt.Errorf("role '%s' not found", roleName)
}

// ValidateRole checks if a role exists in the organization
func (s *EnterpriseConnectionService) ValidateRole(roleName string) error {
	_, err := s.ResolveProvidedRoleNameOrCustomRoleId(roleName)
	return err
}

// DisplayGroupMappingsTable formats and displays group mappings in a table
func (s *EnterpriseConnectionService) DisplayGroupMappingsTable(groupMappings map[string][]string) error {
	var data [][]string

	for roleIdOrName, idpGroups := range groupMappings {
		idpGroupsStr := strings.Join(idpGroups, ", ")
		displayName := s.ResolveRoleDisplayName(roleIdOrName)
		data = append(data, []string{displayName, idpGroupsStr})
	}

	// Sort data by role name (first column)
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	return utils.PrintTable([]string{"Qovery Role", "Your IDPs roles"}, data)
}

// DisplayEnterpriseConnection displays the complete enterprise connection information
func (s *EnterpriseConnectionService) DisplayEnterpriseConnection(connection *qovery.EnterpriseConnectionDto) error {
	// Display connection settings in table format
	defaultRoleDisplay := s.ResolveRoleDisplayName(connection.DefaultRole)
	settingsData := [][]string{
		{defaultRoleDisplay, fmt.Sprintf("%t", connection.EnforceGroupSync)},
	}

	utils.Println("Configuration:")
	utils.Println("=============")
	err := utils.PrintTable([]string{"Default Role", "Enforce Sync Group"}, settingsData)
	if err != nil {
		return err
	}

	utils.Println("Group Mappings:")
	utils.Println("==============")
	return s.DisplayGroupMappingsTable(connection.GroupMappings)
}

// ParseIdpGroupNames parses comma-separated IDP group names
func ParseIdpGroupNames(idpGroupNames string) []string {
	if idpGroupNames == "" {
		return []string{}
	}

	parts := strings.Split(idpGroupNames, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// CreateConnectionUpdateDto creates a DTO for updating enterprise connection
func CreateConnectionUpdateDto(defaultRole string, enforceGroupSync bool, groupMappings map[string][]string) qovery.EnterpriseConnectionDto {
	return qovery.EnterpriseConnectionDto{
		DefaultRole:      defaultRole,
		EnforceGroupSync: enforceGroupSync,
		GroupMappings:    groupMappings,
	}
}
