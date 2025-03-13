// Package permissions provides functionality for checking permission patterns
// against HTTP paths using MQTT-style and NATS-style topic pattern matching.
package permissions

import (
	"strings"
)

// SchemaType represents the type of topic schema (MQTT or NATS)
type SchemaType int

const (
	// MQTT schema uses / as separator, + for single-level, # for multi-level
	MQTT SchemaType = iota
	// NATS schema uses . as separator, * for single-level, > for multi-level
	NATS
)

// Matcher provides functions for topic pattern matching with support
// for both MQTT and NATS pattern formats
type Matcher struct{}

// NewMatcher creates a new topic pattern matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// DetectSchemaType attempts to detect whether a pattern uses MQTT or NATS format
// based on the separators and wildcards used
func (m *Matcher) DetectSchemaType(pattern string) SchemaType {
	// If pattern contains NATS-specific wildcards or separator
	if strings.Contains(pattern, "*") || strings.Contains(pattern, ">") ||
		(strings.Contains(pattern, ".") && !strings.Contains(pattern, "/")) {
		return NATS
	}
	// Default to MQTT
	return MQTT
}

// Match checks if a given path matches a pattern.
// It automatically detects the schema type (MQTT or NATS) and applies
// the appropriate matching rules.
func (m *Matcher) Match(pattern, path string) bool {
	schemaType := m.DetectSchemaType(pattern)
	
	// Normalize the pattern and path according to the schema
	normalizedPattern := m.normalizePath(pattern, schemaType)
	normalizedPath := m.normalizePath(path, schemaType)
	
	// Get the appropriate separator for the schema
	separator := "/"
	if schemaType == NATS {
		separator = "."
	}
	
	// Split pattern and path into segments
	patternParts := strings.Split(normalizedPattern, separator)
	pathParts := strings.Split(normalizedPath, separator)
	
	// Handle special case: multi-level wildcard matches everything
	if normalizedPattern == "#" || normalizedPattern == ">" {
		return true
	}
	
	return m.matchParts(patternParts, pathParts, schemaType)
}

// MapPathToTopic converts an HTTP path to a topic pattern format.
// The format parameter specifies whether to use MQTT or NATS format.
func (m *Matcher) MapPathToTopic(path string, schemaType SchemaType) string {
	// Remove leading slash 
	path = strings.TrimPrefix(path, "/")
	
	// For NATS, replace / with .
	if schemaType == NATS {
		path = strings.ReplaceAll(path, "/", ".")
	}
	
	return path
}

// normalizePath standardizes a path based on the schema type
func (m *Matcher) normalizePath(path string, schemaType SchemaType) string {
	// Remove leading and trailing separators
	path = strings.Trim(path, "/")
	
	if schemaType == NATS {
		path = strings.Trim(path, ".")
	}
	
	return path
}

// matchParts recursively compares pattern segments with path segments
// using the appropriate schema rules
func (m *Matcher) matchParts(patternParts, pathParts []string, schemaType SchemaType) bool {
	// Base case: if no more pattern parts, match only if no more path parts
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	
	// Get the current pattern segment
	segment := patternParts[0]
	
	// Handle multi-level wildcard (# for MQTT, > for NATS)
	multiWildcard := "#"
	if schemaType == NATS {
		multiWildcard = ">"
	}
	
	if segment == multiWildcard {
		// Multi-level wildcard must be the last segment in a valid pattern
		if len(patternParts) > 1 {
			return false // Invalid pattern - multi-wildcard followed by more segments
		}
		return true // Matches any remaining path parts
	}
	
	// No more path parts but still have pattern parts (that aren't multi-wildcard)
	if len(pathParts) == 0 {
		return false
	}
	
	// Handle single-level wildcard (+ for MQTT, * for NATS) or exact match
	singleWildcard := "+"
	if schemaType == NATS {
		singleWildcard = "*"
	}
	
	if segment == singleWildcard || segment == pathParts[0] {
		return m.matchParts(patternParts[1:], pathParts[1:], schemaType)
	}
	
	// No match
	return false
}

// HasPermission determines if a user's role permissions allow access to a specific path
// based on the HTTP method (mapped to publish/subscribe permissions).
// It checks against both MQTT and NATS patterns in the permission lists.
func (m *Matcher) HasPermission(
	path string, 
	method string, 
	publishPermissions []string, 
	subscribePermissions []string,
) bool {
	// Determine which permissions to check based on HTTP method
	var permissions []string
	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		permissions = publishPermissions
	} else {
		permissions = subscribePermissions
	}
	
	// Check each permission pattern
	for _, pattern := range permissions {
		schemaType := m.DetectSchemaType(pattern)
		mqttTopic := m.MapPathToTopic(path, schemaType)
		
		if m.Match(pattern, mqttTopic) {
			return true
		}
	}
	
	return false
}
