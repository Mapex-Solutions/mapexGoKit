package configuration

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
)

// Reads an environment variable and parses it as a bool, or returns the default.
func getEnvBool(key string, defaultValue bool) bool {
	if valueStr, exists := os.LookupEnv(key); exists && valueStr != "" {
		value, err := strconv.ParseBool(valueStr)
		if err != nil {
			log.Printf("Error converting %s to bool: %v. Using default value.", key, err)
			return defaultValue
		}
		return value
	}
	return defaultValue
}

// Reads an environment variable or returns the default string.
func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

// Reads an environment variable and parses it as an int, or returns the default.
func getEnvInt(key string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(key); exists && valueStr != "" {
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			log.Printf("Error converting %s to int: %v. Using default value.", key, err)
			return defaultValue
		}
		return value
	}
	return defaultValue
}

// Reads an environment variable and splits it into a string array, or returns the default.
func getEnvArray(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

// Reads an environment variable and parses it as JSON, or returns the default map.
func getEnvJSON(key string, defaultValue map[string]interface{}) map[string]interface{} {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(value), &parsed)
		if err != nil {
			log.Printf("Error parsing %s: %v. Using default values.", key, err)
			return defaultValue
		}
		return parsed
	}
	return defaultValue
}
