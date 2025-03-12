// Package pocketbase provides a client for interacting with the PocketBase API
// to manage users, roles, and permissions.
package pocketbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// Client is a PocketBase API client
type Client struct {
	baseURL        string
	httpClient     *http.Client
	authToken      string
	logger         *zap.Logger
	userCollection string
	roleCollection string
}

// User represents a user in PocketBase
type User struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	RoleID   string    `json:"role_id"`
	Active   bool      `json:"active"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

// Role represents a role in PocketBase with permissions
type Role struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	PublishPermissions   json.RawMessage `json:"publish_permissions"`
	SubscribePermissions json.RawMessage `json:"subscribe_permissions"`
	Created              time.Time       `json:"created"`
	Updated              time.Time       `json:"updated"`
}

// PocketBaseListResponse represents a generic list response from PocketBase
type PocketBaseListResponse[T any] struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"perPage"`
	TotalItems int    `json:"totalItems"`
	TotalPages int    `json:"totalPages"`
	Items      []T    `json:"items"`
}

// PocketBaseAuthResponse represents an authentication response from PocketBase
type PocketBaseAuthResponse struct {
	Token  string      `json:"token"`
	Record interface{} `json:"record"`
}

// JWTResponse represents a response from the PocketBase auth-refresh endpoint
type JWTResponse struct {
	Token  string `json:"token"`
	Record User   `json:"record"`
}

// NewClient creates a new PocketBase client
func NewClient(baseURL, userCollection, roleCollection string, logger *zap.Logger) *Client {
	return &Client{
		baseURL:        baseURL,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		logger:         logger,
		userCollection: userCollection,
		roleCollection: roleCollection,
	}
}

// Authenticate authenticates with PocketBase using admin credentials
func (c *Client) Authenticate(email, password string) error {
	data := map[string]string{
		"identity": email,
		"password": password,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	authEndpoint := fmt.Sprintf("%s/api/collections/_superusers/auth-with-password", c.baseURL)
	c.logger.Debug("Authenticating with PocketBase", zap.String("endpoint", authEndpoint))

	req, err := http.NewRequest("POST", authEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp PocketBaseAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.authToken = authResp.Token
	c.logger.Info("Successfully authenticated with PocketBase")
	return nil
}

// GetAllUsers retrieves all users from PocketBase
func (c *Client) GetAllUsers() ([]User, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, c.userCollection)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	query := reqURL.Query()
	query.Set("filter", "active=true")
	query.Set("perPage", "200") // Adjust based on expected user count
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create users request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send users request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var usersResp PocketBaseListResponse[User]
	if err := json.Unmarshal(body, &usersResp); err != nil {
		return nil, fmt.Errorf("failed to decode users response: %w", err)
	}

	c.logger.Info("Retrieved users from PocketBase", zap.Int("count", len(usersResp.Items)))
	return usersResp.Items, nil
}

// GetAllRoles retrieves all roles from PocketBase
func (c *Client) GetAllRoles() ([]Role, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records", c.baseURL, c.roleCollection)
	
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create roles request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send roles request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("roles request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rolesResp PocketBaseListResponse[Role]
	if err := json.Unmarshal(body, &rolesResp); err != nil {
		return nil, fmt.Errorf("failed to decode roles response: %w", err)
	}

	c.logger.Info("Retrieved roles from PocketBase", zap.Int("count", len(rolesResp.Items)))
	return rolesResp.Items, nil
}

// GetUserByToken validates a JWT token and retrieves the associated user
// This uses PocketBase's auth-refresh endpoint to validate the token
func (c *Client) GetUserByToken(token string) (*User, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("client not authenticated")
	}

	// Use PocketBase's JWT verification via auth-refresh endpoint
	endpoint := fmt.Sprintf("%s/api/collections/%s/auth-refresh", c.baseURL, c.userCollection)
	
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create token validation request: %w", err)
	}

	// Set the user's token in the Authorization header
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token validation request: %w", err)
	}
	defer resp.Body.Close()

	// If response is not 200 OK, the token is invalid
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token validation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var jwtResp JWTResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwtResp); err != nil {
		return nil, fmt.Errorf("failed to decode token validation response: %w", err)
	}

	// Check if the user is active
	if !jwtResp.Record.Active {
		return nil, fmt.Errorf("user account is inactive")
	}

	c.logger.Debug("Successfully validated user token", 
		zap.String("user_id", jwtResp.Record.ID),
		zap.String("username", jwtResp.Record.Username))
	
	return &jwtResp.Record, nil
}

// GetRoleByID retrieves a role by its ID
func (c *Client) GetRoleByID(id string) (*Role, error) {
	if c.authToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	endpoint := fmt.Sprintf("%s/api/collections/%s/records/%s", c.baseURL, c.roleCollection, id)
	
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create role request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send role request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("role request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var role Role
	if err := json.NewDecoder(resp.Body).Decode(&role); err != nil {
		return nil, fmt.Errorf("failed to decode role response: %w", err)
	}

	return &role, nil
}

// GetPublishPermissions extracts the string array from JSON field
func (r *Role) GetPublishPermissions() ([]string, error) {
	var permissions []string
	if len(r.PublishPermissions) == 0 {
		return permissions, nil
	}
	
	if err := json.Unmarshal(r.PublishPermissions, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// GetSubscribePermissions extracts the string array from JSON field
func (r *Role) GetSubscribePermissions() ([]string, error) {
	var permissions []string
	if len(r.SubscribePermissions) == 0 {
		return permissions, nil
	}
	
	if err := json.Unmarshal(r.SubscribePermissions, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}
