// Package gateway implements the core API gateway functionality
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"api-gateway/internal/cache"
	"api-gateway/internal/config"
	"api-gateway/internal/metrics"
	"api-gateway/internal/pocketbase"
	"api-gateway/pkg/permissions"
)

// ApiGateway represents the API gateway service
type ApiGateway struct {
	router       *chi.Mux
	logger       *zap.Logger
	pbClient     *pocketbase.Client
	cache        *cache.Cache
	metrics      *metrics.Metrics
	routes       []config.Route
	cacheTTL     time.Duration
	permMatcher  *permissions.Matcher
}

// New creates a new API gateway
func New(cfg *config.Config, logger *zap.Logger) (*ApiGateway, error) {
	// Initialize the metrics
	m := metrics.NewMetrics("api_gateway")
	
	// Initialize the PocketBase client
	pbClient := pocketbase.NewClient(
		cfg.PocketBase.URL,
		cfg.PocketBase.UserCollection,
		cfg.PocketBase.RoleCollection,
		logger.With(zap.String("component", "pocketbase")),
	)
	
	// Authenticate with PocketBase
	if err := pbClient.Authenticate(cfg.PocketBase.ServiceAccount, cfg.PocketBase.ServicePassword); err != nil {
		return nil, fmt.Errorf("failed to authenticate with PocketBase: %w", err)
	}
	
	// Initialize the cache
	cacheComponent := cache.New(
		time.Duration(cfg.CacheTTLSeconds)*time.Second,
		logger.With(zap.String("component", "cache")),
	)
	
	// Initialize the permission matcher
	permMatcher := permissions.NewMatcher()
	
	// Create the gateway
	gw := &ApiGateway{
		router:       chi.NewRouter(),
		logger:       logger,
		pbClient:     pbClient,
		cache:        cacheComponent,
		metrics:      m,
		routes:       cfg.Routes,
		cacheTTL:     time.Duration(cfg.CacheTTLSeconds) * time.Second,
		permMatcher:  permMatcher,
	}
	
	// Set up router middleware
	gw.router.Use(middleware.RequestID)
	gw.router.Use(middleware.RealIP)
	gw.router.Use(gw.loggingMiddleware)
	gw.router.Use(middleware.Recoverer)
	gw.router.Use(middleware.Timeout(30 * time.Second))
	gw.router.Use(gw.metricsMiddleware)
	
	// Set up routes
	gw.router.Get("/health", gw.handleHealth)
	gw.router.Handle("/metrics", promhttp.Handler())
	
	// Set up proxy routes
	if err := gw.setupProxyRoutes(); err != nil {
		return nil, fmt.Errorf("failed to set up proxy routes: %w", err)
	}
	
	// Preload cache
	if err := gw.refreshCache(); err != nil {
		logger.Warn("Failed to preload cache, will retry on first request", zap.Error(err))
	}
	
	return gw, nil
}

// ServeHTTP implements the http.Handler interface
func (g *ApiGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.router.ServeHTTP(w, r)
}

// refreshCache refreshes the user and role caches from PocketBase
func (g *ApiGateway) refreshCache() error {
	// Check if refresh is needed
	if !g.cache.RefreshIfNeeded() {
		return nil
	}
	
	g.logger.Debug("Refreshing cache from PocketBase")
	
	// Get all roles
	roles, err := g.pbClient.GetAllRoles()
	if err != nil {
		return fmt.Errorf("failed to get roles: %w", err)
	}
	
	// Get all users
	users, err := g.pbClient.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}
	
	// Update the cache
	g.cache.BulkLoadRoles(roles)
	g.cache.BulkLoadUsers(users)
	
	// Update metrics
	stats := g.cache.GetStats()
	g.metrics.UpdateCacheSize(stats["users"], stats["roles"])
	g.metrics.RecordCacheRefresh()
	
	g.logger.Info("Cache refreshed", 
		zap.Int("users", stats["users"]), 
		zap.Int("roles", stats["roles"]))
	
	return nil
}

// authMiddleware authenticates and authorizes requests
func (g *ApiGateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get start time for metrics
		startTime := time.Now()
		
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			g.metrics.RecordAuthFailure("missing_token")
			g.sendError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}
		
		// Format should be "Bearer {token}"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			g.metrics.RecordAuthFailure("invalid_token_format")
			g.sendError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}
		
		token := parts[1]
		
		// Refresh cache if needed
		if err := g.refreshCache(); err != nil {
			g.logger.Error("Failed to refresh cache", zap.Error(err))
			g.sendError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		
		// Try to get user from cache by token fingerprint (first 8 chars)
		tokenKey := token
		if len(token) > 8 {
			tokenKey = token[:8] + "..." // We use partial token as cache key for security
		}
		
		user := g.cache.GetUserByToken(tokenKey)
		if user == nil {
			// User not in cache, validate token with PocketBase
			fetchedUser, err := g.pbClient.GetUserByToken(token)
			if err != nil {
				g.logger.Debug("Token validation failed", 
					zap.Error(err), 
					zap.String("token_prefix", tokenKey))
				g.metrics.RecordAuthFailure("invalid_token")
				g.sendError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			
			// Add user to cache
			user = fetchedUser
			g.cache.AddUser(tokenKey, user)
		}
		
		// No need to check if user is active - already checked in GetUserByToken
		
		// Get role from cache
		role := g.cache.GetRoleByID(user.RoleID)
		if role == nil {
			// Role not in cache, try to get from PocketBase
			fetchedRole, err := g.pbClient.GetRoleByID(user.RoleID)
			if err != nil {
				g.logger.Error("Failed to get role", 
					zap.Error(err), 
					zap.String("role_id", user.RoleID),
					zap.String("username", user.Username))
				g.metrics.RecordAuthFailure("role_not_found")
				g.sendError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			
			// Add role to cache
			role = fetchedRole
			g.cache.AddRole(role.ID, role)
		}
		
		// Get role permissions
		publishPermissions, err := role.GetPublishPermissions()
		if err != nil {
			g.logger.Error("Failed to parse publish permissions", 
				zap.Error(err), 
				zap.String("role", role.Name))
			g.metrics.RecordAuthFailure("invalid_permissions")
			g.sendError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		
		subscribePermissions, err := role.GetSubscribePermissions()
		if err != nil {
			g.logger.Error("Failed to parse subscribe permissions", 
				zap.Error(err), 
				zap.String("role", role.Name))
			g.metrics.RecordAuthFailure("invalid_permissions")
			g.sendError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		
		// Extract the top-level prefix from the path for better debug logging
		pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
		topLevelPrefix := ""
		if len(pathParts) > 0 {
			topLevelPrefix = pathParts[0]
		}
		
		// Check if user has permission to access this path
		if !g.permMatcher.HasPermission(r.URL.Path, r.Method, publishPermissions, subscribePermissions) {
			g.logger.Debug("Permission denied",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("top_level_prefix", topLevelPrefix),
				zap.Strings("publish_permissions", publishPermissions),
				zap.Strings("subscribe_permissions", subscribePermissions))
				
			g.metrics.RecordAuthFailure("insufficient_permissions")
			g.sendError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		
		g.logger.Debug("Permission granted",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.String("top_level_prefix", topLevelPrefix),
			zap.String("username", user.Username))
		
		// Add user and role to request context
		ctx := context.WithValue(r.Context(), "user", user)
		ctx = context.WithValue(ctx, "role", role)
		
		// Record request duration for auth processing
		g.metrics.ObserveRequestDuration(r.Method, "auth_processing", time.Since(startTime).Seconds())
		
		// Call the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// setupProxyRoutes configures the proxy routes from the configuration
func (g *ApiGateway) setupProxyRoutes() error {
	// Create a route map for faster lookups
	routeMap := make(map[string]*http.Handler)
	
	// First, set up all the proxy handlers
	for _, route := range g.routes {
		targetURL, err := url.Parse(route.TargetURL)
		if err != nil {
			return fmt.Errorf("invalid target URL %s: %w", route.TargetURL, err)
		}
		
		g.logger.Info("Setting up proxy route", 
			zap.String("pathPrefix", route.PathPrefix),
			zap.String("targetURL", route.TargetURL),
			zap.Bool("stripPrefix", route.StripPrefix))
		
		// Create a reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		
		// Store original director function
		originalDirector := proxy.Director
		
		// Create a custom director function
		proxy.Director = func(req *http.Request) {
			// Call the original director
			originalDirector(req)
			
			// Strip the prefix if configured
			if route.StripPrefix {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, route.PathPrefix)
				if !strings.HasPrefix(req.URL.Path, "/") {
					req.URL.Path = "/" + req.URL.Path
				}
			}
			
			// Forward the user ID if available
			if user, ok := req.Context().Value("user").(*pocketbase.User); ok {
				req.Header.Set("X-User-ID", user.ID)
				req.Header.Set("X-Username", user.Username)
			}
			
			// Forward the role if available
			if role, ok := req.Context().Value("role").(*pocketbase.Role); ok {
				req.Header.Set("X-Role-ID", role.ID)
				req.Header.Set("X-Role-Name", role.Name)
			}
			
			g.logger.Debug("Proxying request", 
				zap.String("path", req.URL.Path),
				zap.String("target", targetURL.String()))
		}
		
		// Set up error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			g.logger.Error("Proxy error",
				zap.Error(err),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method))
			
			g.sendError(w, http.StatusBadGateway, "backend service error")
		}
		
		// Store the proxy handler in our map
		handler := http.Handler(proxy)
		routeMap[route.PathPrefix] = &handler
	}
	
	// Apply global authentication middleware to all requests except system endpoints (/health, /metrics)
	g.router.Group(func(r chi.Router) {
		r.Use(g.authMiddleware)
		
		// Register specific routes
		for _, route := range g.routes {
			if handler, ok := routeMap[route.PathPrefix]; ok {
				r.Handle(route.PathPrefix+"*", *handler)
			}
		}
		
		// Add a catch-all route for any path that doesn't match defined routes
		// This ensures that paths like /acm/... are properly rejected with 403 if not authorized
		r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
			// If we reach here, it means the path didn't match any defined route
			// The authMiddleware would have already checked permissions and rejected
			// unauthorized requests, so this is a fallback for paths that aren't configured
			g.logger.Warn("Request to undefined route", 
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method))
			g.sendError(w, http.StatusNotFound, "no route configured for this path")
		})
	})
	
	return nil
}

// handleHealth handles health check requests
func (g *ApiGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check PocketBase connection
	pbStatus := "ok"
	if _, err := g.pbClient.GetAllRoles(); err != nil {
		pbStatus = "error: " + err.Error()
	}
	
	// Check cache status
	cacheStats := g.cache.GetStats()
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"status": "ok",
		"components": map[string]string{
			"pocketbase": pbStatus,
		},
		"cache": cacheStats,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	json.NewEncoder(w).Encode(response)
}

// loggingMiddleware logs information about each request
func (g *ApiGateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap the response writer to capture the status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		
		// Process the request
		next.ServeHTTP(ww, r)
		
		// Log the request
		duration := time.Since(start)
		
		// Extract request ID if available
		requestID := middleware.GetReqID(r.Context())
		
		// Determine log level based on status code
		if ww.Status() >= 500 {
			g.logger.Error("Request completed with server error",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", duration),
				zap.String("request_id", requestID))
		} else if ww.Status() >= 400 {
			g.logger.Warn("Request completed with client error",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", duration),
				zap.String("request_id", requestID))
		} else {
			g.logger.Info("Request completed successfully",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", duration),
				zap.String("request_id", requestID))
		}
	})
}

// metricsMiddleware collects metrics for each request
func (g *ApiGateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap the response writer to capture the status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		
		// Increment active connections
		g.metrics.IncActiveConnections()
		defer g.metrics.DecActiveConnections()
		
		// Process the request
		next.ServeHTTP(ww, r)
		
		// Record metrics
		duration := time.Since(start).Seconds()
		g.metrics.RecordRequest(r.Method, r.URL.Path, ww.Status())
		g.metrics.ObserveRequestDuration(r.Method, r.URL.Path, duration)
	})
}

// sendError sends a JSON error response
func (g *ApiGateway) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	response := map[string]interface{}{
		"error": message,
		"status": status,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	json.NewEncoder(w).Encode(response)
}
