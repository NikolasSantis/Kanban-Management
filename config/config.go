package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"

	"github.com/joho/godotenv"
)

const minJWTSecretLength = 32

type Config struct {
	AppEnv              string
	Port                string
	MongoURI            string
	DatabaseName        string
	TestDatabaseName    string
	JWTSecret           string
	JWTExpiresHours     int
	LogLevel            string
	LogFormat           string
	AttachmentMaxSizeMB int
}

var current *Config

// Load lê o .env (opcional) e as variáveis de ambiente, valida e guarda a
// configuração para ser acessada por Get(). Variáveis do ambiente real têm
// precedência sobre o .env.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("erro ao ler .env: %w", err)
	}

	var errs []error

	cfg := &Config{
		AppEnv:       getEnv("APP_ENV", "development"),
		Port:         getEnv("PORT", "8000"),
		MongoURI:     os.Getenv("MONGODB_URI"),
		DatabaseName: os.Getenv("DATABASE_NAME"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
	}

	cfg.TestDatabaseName = getEnv("TEST_DATABASE_NAME", cfg.DatabaseName+"_test")

	defaultLogFormat := "text"
	if cfg.IsProduction() {
		defaultLogFormat = "json"
	}
	cfg.LogFormat = getEnv("LOG_FORMAT", defaultLogFormat)

	var err error

	if cfg.JWTExpiresHours, err = getEnvInt("JWT_EXPIRES_HOURS", 24); err != nil {
		errs = append(errs, err)
	}

	if cfg.AttachmentMaxSizeMB, err = getEnvInt("ATTACHMENT_MAX_SIZE_MB", 10); err != nil {
		errs = append(errs, err)
	}

	var missing []string
	for _, key := range []string{"MONGODB_URI", "DATABASE_NAME", "JWT_SECRET"} {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("variáveis obrigatórias ausentes: %v", missing))
	}

	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < minJWTSecretLength {
		errs = append(errs, fmt.Errorf("JWT_SECRET deve ter pelo menos %d caracteres", minJWTSecretLength))
	}

	if !slices.Contains([]string{"development", "test", "production"}, cfg.AppEnv) {
		errs = append(errs, fmt.Errorf("APP_ENV inválido %q (use development, test ou production)", cfg.AppEnv))
	}

	if !slices.Contains([]string{"debug", "info", "warn", "error"}, cfg.LogLevel) {
		errs = append(errs, fmt.Errorf("LOG_LEVEL inválido %q (use debug, info, warn ou error)", cfg.LogLevel))
	}

	if !slices.Contains([]string{"text", "json"}, cfg.LogFormat) {
		errs = append(errs, fmt.Errorf("LOG_FORMAT inválido %q (use text ou json)", cfg.LogFormat))
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	current = cfg

	return cfg, nil
}

// Get retorna a configuração carregada. Chamar antes de Load é erro de programação.
func Get() *Config {
	if current == nil {
		panic("config: Get() chamado antes de config.Load()")
	}

	return current
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func (c *Config) IsTest() bool {
	return c.AppEnv == "test"
}

func getEnv(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return def
}

func getEnvInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s deve ser um inteiro maior que zero (recebido %q)", key, raw)
	}

	return value, nil
}
