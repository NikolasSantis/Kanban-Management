package config

import (
	"strings"
	"testing"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("DATABASE_NAME", "kanban")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
}

func clearOptionalEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"APP_ENV", "PORT", "TEST_DATABASE_NAME", "JWT_EXPIRES_HOURS", "LOG_LEVEL", "LOG_FORMAT", "ATTACHMENT_MAX_SIZE_MB"} {
		t.Setenv(k, "")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("DATABASE_NAME", "")
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, k := range []string{"MONGODB_URI", "DATABASE_NAME", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("erro deveria citar %s: %v", k, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8000" || cfg.JWTExpiresHours != 24 || cfg.AppEnv != "development" ||
		cfg.LogFormat != "text" || cfg.TestDatabaseName != "kanban_test" || cfg.AttachmentMaxSizeMB != 10 {
		t.Errorf("defaults inesperados: %+v", cfg)
	}
}

func TestLoadProductionDefaultsToJSONLog(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)
	t.Setenv("APP_ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogFormat != "json" || !cfg.IsProduction() {
		t.Errorf("esperava json em produção: %+v", cfg)
	}
}

func TestLoadInvalidInt(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)

	for _, v := range []string{"abc", "0", "-1"} {
		t.Setenv("JWT_EXPIRES_HOURS", v)
		if _, err := Load(); err == nil {
			t.Errorf("JWT_EXPIRES_HOURS=%q deveria falhar", v)
		}
	}
}

func TestLoadShortSecret(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)
	t.Setenv("JWT_SECRET", "abc")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("esperava erro de JWT_SECRET, veio %v", err)
	}
}

func TestLoadTestEnvDefaultsToErrorLogLevel(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)
	t.Setenv("APP_ENV", "test")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("esperava error em APP_ENV=test, veio %q", cfg.LogLevel)
	}
}

func TestLoadStorageDefaultsAndValidation(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)
	t.Setenv("STORAGE_DRIVER", "")
	t.Setenv("STORAGE_LOCAL_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageDriver != "local" || cfg.StorageLocalPath != "./uploads" {
		t.Errorf("defaults de storage inesperados: %+v", cfg)
	}

	t.Setenv("STORAGE_DRIVER", "memory")
	cfg, err = Load()
	if err != nil || cfg.StorageDriver != "memory" {
		t.Errorf("memory deveria ser aceito: %v %+v", err, cfg)
	}

	t.Setenv("STORAGE_DRIVER", "s3")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "STORAGE_DRIVER") {
		t.Errorf("driver inválido deveria falhar: %v", err)
	}
}

func TestLoadSwaggerEnabled(t *testing.T) {
	setValidEnv(t)
	clearOptionalEnv(t)

	t.Setenv("SWAGGER_ENABLED", "")
	cfg, err := Load()
	if err != nil || !cfg.SwaggerEnabled {
		t.Fatalf("o padrão é true: %v %+v", err, cfg)
	}

	t.Setenv("SWAGGER_ENABLED", "false")
	cfg, err = Load()
	if err != nil || cfg.SwaggerEnabled {
		t.Fatalf("false deveria desligar: %v %+v", err, cfg)
	}

	t.Setenv("SWAGGER_ENABLED", "talvez")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SWAGGER_ENABLED") {
		t.Fatalf("valor inválido deveria falhar: %v", err)
	}
}
