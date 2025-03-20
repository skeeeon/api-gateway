// Package cache provides in-memory caching for user and role data
// with automatic expiration to minimize database lookups
package cache

import (
	"sync"
	"time"

	"api-gateway/internal/pocketbase"
	"go.uber.org/zap"
)

// Cache is an in-memory cache for user and role data
type Cache struct {
	userCache       map[string]*pocketbase.User // Map hashed token -> User
	roleCache       map[string]*pocketbase.Role // Map ID -> Role
	mutex           sync.RWMutex
	ttl             time.Duration
	lastRefreshTime time.Time
	logger          *zap.Logger
	tokenHasher     *TokenHasher
}

// New creates a new cache with the specified TTL
func New(ttl time.Duration, logger *zap.Logger) *Cache {
	return &Cache{
		userCache:   make(map[string]*pocketbase.User),
		roleCache:   make(map[string]*pocketbase.Role),
		ttl:         ttl,
		logger:      logger,
		tokenHasher: NewTokenHasher(),
	}
}

// GetUserByToken retrieves a user from the cache by token
// The token is hashed before lookup to avoid storing raw tokens
// Returns nil if the user is not in the cache
func (c *Cache) GetUserByToken(token string) *pocketbase.User {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	// Hash the token to get the cache key
	hashedToken := c.tokenHasher.HashToken(token)
	
	user, found := c.userCache[hashedToken]
	if !found {
		return nil
	}
	return user
}

// GetRoleByID retrieves a role from the cache by its ID
// Returns nil if the role is not in the cache
func (c *Cache) GetRoleByID(id string) *pocketbase.Role {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	role, found := c.roleCache[id]
	if !found {
		return nil
	}
	return role
}

// AddUser adds or updates a user in the cache
// The token is hashed before being used as a key for security
func (c *Cache) AddUser(token string, user *pocketbase.User) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Hash the token to get the cache key
	hashedToken := c.tokenHasher.HashToken(token)
	
	c.userCache[hashedToken] = user
	c.logger.Debug("Added user to cache", 
		zap.String("username", user.Username), 
		zap.String("hashed_token", hashedToken[:8]+"...")) // Log prefix of hash for debugging
}

// AddRole adds or updates a role in the cache
func (c *Cache) AddRole(id string, role *pocketbase.Role) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.roleCache[id] = role
	c.logger.Debug("Added role to cache", zap.String("role", role.Name))
}

// ClearCache clears all cached users and roles
func (c *Cache) ClearCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.userCache = make(map[string]*pocketbase.User)
	c.roleCache = make(map[string]*pocketbase.Role)
	c.lastRefreshTime = time.Now()
	
	c.logger.Debug("Cache cleared")
}

// RefreshIfNeeded refreshes the cache if the TTL has expired
// Returns true if the cache needed refreshing
func (c *Cache) RefreshIfNeeded() bool {
	c.mutex.RLock()
	needsRefresh := time.Since(c.lastRefreshTime) > c.ttl
	c.mutex.RUnlock()
	
	if needsRefresh {
		c.logger.Debug("Cache TTL expired, refreshing", 
			zap.Duration("ttl", c.ttl),
			zap.Time("lastRefresh", c.lastRefreshTime))
		c.ClearCache()
		return true
	}
	
	return false
}

// BulkLoadUsers loads multiple users into the cache at once
// Note: For JWT implementation, this is now primarily used for pre-warming the role cache
// Individual users are cached on-demand as they authenticate
func (c *Cache) BulkLoadUsers(users []pocketbase.User) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Since we can't pre-cache users by JWT tokens (they're dynamic),
	// we primarily load roles here, but still track active user count
	activeUserCount := 0
	for i := range users {
		if users[i].Active {
			activeUserCount++
		}
	}
	
	c.logger.Debug("Processed active users", zap.Int("count", activeUserCount))
}

// BulkLoadRoles loads multiple roles into the cache at once
func (c *Cache) BulkLoadRoles(roles []pocketbase.Role) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	for i := range roles {
		c.roleCache[roles[i].ID] = &roles[i]
	}
	
	c.logger.Debug("Bulk loaded roles into cache", zap.Int("count", len(roles)))
}

// GetStats returns statistics about the cache
func (c *Cache) GetStats() map[string]int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return map[string]int{
		"users": len(c.userCache),
		"roles": len(c.roleCache),
	}
}
