// Package config handles application configuration
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"server"`
	
	PocketBase struct {
		URL            string `mapstructure:"url"`
		ServiceAccount string `mapstructure:"serviceAccount"`
		ServicePassword string `mapstructure:"servicePassword"`
		UserCollection string `mapstructure:"userCollection"`
		RoleCollection string `mapstructure:"roleCollection"`
	} `mapstructure:"pocketbase"`
	
	Routes          []Route `mapstructure:"routes"`
	LogLevel        string  `mapstructure:"logLevel"`
	CacheTTLSeconds int     `mapstructure:"cacheTTLSeconds"`
}

// Route defines a proxy route
type Route struct {
	PathPrefix  string `mapstructure:"pathPrefix"`
	TargetURL   string `mapstructure:"targetUrl"`
	StripPrefix bool   `mapstructure:"stripPrefix"`
}

// LoadConfig loads the application configuration from file and environment variables
func LoadConfig(configPath string, logger *zap.Logger) (*Config, error) {
	v := viper.New()
	
	// Set default values
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9000)
	v.SetDefault("pocketbase.userCollection", "mqtt_users")
	v.SetDefault("pocketbase.roleCollection", "mqtt_roles")
	v.SetDefault("logLevel", "info")
	v.SetDefault("cacheTTLSeconds", 300)
	
	// Configure file path
	if configPath != "" {
		// Use provided config file
		v.SetConfigFile(configPath)
	} else {
		// Search for config in default locations
		v.SetConfigName("config")
		v.SetConfigType("json")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/api-gateway")
	}
	
	// Read environment variables prefixed with "API_GATEWAY_"
	v.SetEnvPrefix("API_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	// Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok && configPath == "" {
			logger.Warn("No configuration file found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		logger.Info("Using config file", zap.String("file", v.ConfigFileUsed()))
	}
	
	// Unmarshal the configuration
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}
	
	// Validate the configuration
	if err := validateConfig(&config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// validateConfig checks if the configuration is valid
func validateConfig(config *Config) error {
	// Check PocketBase URL
	if config.PocketBase.URL == "" {
		return fmt.Errorf("pocketbase.url is required")
	}
	
	// Check PocketBase credentials
	if config.PocketBase.ServiceAccount == "" {
		return fmt.Errorf("pocketbase.serviceAccount is required")
	}
	
	if config.PocketBase.ServicePassword == "" {
		return fmt.Errorf("pocketbase.servicePassword is required")
	}
	
	// Check if at least one route is defined
	if len(config.Routes) == 0 {
		return fmt.Errorf("at least one route must be defined")
	}
	
	// Check each route
	for i, route := range config.Routes {
		if route.PathPrefix == "" {
			return fmt.Errorf("routes[%d].pathPrefix is required", i)
		}
		
		if route.TargetURL == "" {
			return fmt.Errorf("routes[%d].targetUrl is required", i)
		}
	}
	
	return nil
}

// LoadRoutes loads routes from a separate configuration file
func LoadRoutes(routesPath string) ([]Route, error) {
	v := viper.New()
	
	v.SetConfigFile(routesPath)
	
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading routes file: %w", err)
	}
	
	var routes []Route
	if err := v.UnmarshalKey("routes", &routes); err != nil {
		return nil, fmt.Errorf("unable to decode routes: %w", err)
	}
	
	return routes, nil
}

// GetConfigPath gets the configuration file path from command line arguments
func GetConfigPath() string {
	// Check if a config file was specified as a command line argument
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	
	return ""
}
