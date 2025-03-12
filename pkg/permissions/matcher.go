// Package permissions provides functionality for checking permission patterns
// against HTTP paths using MQTT-style topic pattern matching.
package permissions

import (
	"strings"
)

// Matcher provides functions for MQTT-style topic pattern matching
type Matcher struct {}

// NewMatcher creates a new topic pattern matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// Match checks if a given path matches a pattern. 
// It follows MQTT topic matching rules:
// - '+' matches exactly one path segment
// - '#' matches zero or more path segments (must be at the end of the pattern)
func (m *Matcher) Match(pattern, path string) bool {
	// Standardize the path and pattern by removing leading/trailing slashes
	// and convert to MQTT-style topic format (/ becomes .)
	pattern = m.normalizePath(pattern)
	path = m.normalizePath(path)
	
	// Split pattern and path into segments
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	
	// Handle special case: "#" matches everything
	if pattern == "#" {
		return true
	}
	
	return m.matchParts(patternParts, pathParts)
}

// mapPathToTopic converts an HTTP path to an MQTT topic pattern.
// This is used to transform incoming HTTP requests to a format that can
// be matched against stored permissions.
func (m *Matcher) MapPathToTopic(path string) string {
	// Remove leading slash and standardize format
	return m.normalizePath(path)
}

// normalizePath standardizes a path by removing leading/trailing slashes
func (m *Matcher) normalizePath(path string) string {
	// Remove leading and trailing slashes
	path = strings.Trim(path, "/")
	return path
}

// matchParts recursively compares pattern segments with path segments
func (m *Matcher) matchParts(patternParts, pathParts []string) bool {
	// Base case: if no more pattern parts, match only if no more path parts
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	
	// Get the current pattern segment
	segment := patternParts[0]
	
	// Handle multi-level wildcard '#' (must be the last segment)
	if segment == "#" {
		// '#' must be the last segment in a valid pattern
		if len(patternParts) > 1 {
			return false // Invalid pattern - # followed by more segments
		}
		return true // # matches any remaining path parts
	}
	
	// No more path parts but still have pattern parts (that aren't #)
	if len(pathParts) == 0 {
		return false
	}
	
	// Handle single-level wildcard '+' or exact match
	if segment == "+" || segment == pathParts[0] {
		return m.matchParts(patternParts[1:], pathParts[1:])
	}
	
	// No match
	return false
}

// HasPermission determines if a user's role permissions allow access to a specific path
// based on the HTTP method (mapped to publish/subscribe permissions)
func (m *Matcher) HasPermission(
	path string, 
	method string, 
	publishPermissions []string, 
	subscribePermissions []string,
) bool {
	// Map the HTTP path to MQTT topic pattern
	mqttTopic := m.MapPathToTopic(path)
	
	// Determine which permissions to check based on HTTP method
	var permissions []string
	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		permissions = publishPermissions
	} else {
		permissions = subscribePermissions
	}
	
	// Check each permission pattern
	for _, pattern := range permissions {
		if m.Match(pattern, mqttTopic) {
			return true
		}
	}
	
	return false
}
