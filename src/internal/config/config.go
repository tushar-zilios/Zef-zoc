package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL           string
	JWTSecret             string
	JWTExpirationHours    int
	Port                  string
	GCPServiceAccountJSON string
	GCSBucketName         string
	InternalServiceSecret string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	expHours := 24
	if v := os.Getenv("JWT_EXPIRATION_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			expHours = n
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	return &Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		JWTExpirationHours:    expHours,
		Port:                  port,
		GCPServiceAccountJSON: os.Getenv("GCP_SERVICE_ACCOUNT_JSON"),
		GCSBucketName:         os.Getenv("GCS_BUCKET_NAME"),
		InternalServiceSecret: os.Getenv("INTERNAL_SERVICE_SECRET"),
	}, nil
}
