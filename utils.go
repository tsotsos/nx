package main

import (
	"os"
)

// New Config struct
func NewConfig() *Config {
	return &Config{
		Protocol: getEnv("NX_PROTOCOL", ""),
		Host:     getEnv("NX_HOST", ""),
		Name:     getEnv("NX_NANE", ""),
		User:     getEnv("NX_USER", ""),
		Pin:      getEnv("NX_PIN", ""),
		Url:      getEnv("NX_PROTOCOL", "") + "://" + getEnv("NX_HOST", "") + "/",
	}
}

// returns Environment (string) variable or default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}
