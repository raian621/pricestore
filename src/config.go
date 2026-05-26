package main

import (
	"log"
	"os"
	"strconv"

	secret "github.com/andrewbenton/go-secrets"
	"github.com/joho/godotenv"
)

type Config struct {
	BootstrapDb     bool
	DbHost          string
	DbPort          uint
	DbName          string
	DbUser          string
	DbPassword      secret.Secret[string]
	DbMigrationPath string
	AlpacaApiKey    secret.Secret[string]
	AlpacaSecret    secret.Secret[string]
	AlpacaApiUrl    string
}

func LoadConfigFromEnv(config *Config) error {
	godotenv.Load()

	config.BootstrapDb = envBool("BOOTSTRAP_DB", false)
	config.DbHost = envString("DB_HOST", "localhost")
	config.DbPort = envUint("DB_PORT", 5432)
	config.DbName = envString("DB_NAME", "postgres")
	config.DbUser = envString("DB_USER", "postgres")
	config.DbPassword = secret.Make(envString("DB_PASSWORD", "postgres"))
	config.DbMigrationPath = envString("DB_MIGRATION_PATH",
		"./scripts/migrations/")
	config.AlpacaApiKey = secret.Make(envString("ALPACA_API_KEY", ""))
	config.AlpacaSecret = secret.Make(envString("ALPACA_SECRET", ""))
	config.AlpacaApiUrl = envString("ALPACA_API_URL",
		"https://data.alpaca.markets/v2")

	return nil
}

func envBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err != nil {
			log.Fatalln(err)
		} else {
			return b
		}
	}
	return defaultVal
}

func envUint(key string, defaultVal uint) uint {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseUint(val, 10, 16); err != nil {
			log.Fatalln(err)
		} else {
			return uint(i)
		}
	}
	return defaultVal
}

func envString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
