package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Address != ":9000" {
		t.Errorf("Expected default address ':9000', got '%s'", cfg.Server.Address)
	}

	if cfg.Server.TLS.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}

	if cfg.Storage.RootPath != "./data" {
		t.Errorf("Expected default root path './data', got '%s'", cfg.Storage.RootPath)
	}

	if cfg.Auth.AccessKey != "porterfs" {
		t.Errorf("Expected default access key 'porterfs', got '%s'", cfg.Auth.AccessKey)
	}

	if cfg.Auth.SecretKey != "porterfs" {
		t.Errorf("Expected default secret key 'porterfs', got '%s'", cfg.Auth.SecretKey)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Logging.Level)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "porter-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("NonExistentFile", func(t *testing.T) {
		cfg, err := Load(filepath.Join(tmpDir, "nonexistent.yaml"))
		if err != nil {
			t.Errorf("Expected no error for non-existent file, got: %v", err)
		}
		if cfg == nil {
			t.Error("Expected default config for non-existent file")
		}
	})

	t.Run("ValidConfigFile", func(t *testing.T) {
		configContent := `
server:
  address: ":8080"
  tls:
    enabled: true
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"

storage:
  root_path: "/custom/path"
  max_size_bytes: 1000000

auth:
  access_key: "custom-access"
  secret_key: "custom-secret"

logging:
  level: "debug"
  format: "text"
`
		configFile := filepath.Join(tmpDir, "test-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(configFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.Server.Address != ":8080" {
			t.Errorf("Expected address ':8080', got '%s'", cfg.Server.Address)
		}

		if !cfg.Server.TLS.Enabled {
			t.Error("Expected TLS to be enabled")
		}

		if cfg.Storage.RootPath != "/custom/path" {
			t.Errorf("Expected root path '/custom/path', got '%s'", cfg.Storage.RootPath)
		}

		if cfg.Auth.AccessKey != "custom-access" {
			t.Errorf("Expected access key 'custom-access', got '%s'", cfg.Auth.AccessKey)
		}

		if cfg.Logging.Level != "debug" {
			t.Errorf("Expected log level 'debug', got '%s'", cfg.Logging.Level)
		}
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		invalidContent := `
server:
  address: ":8080"
  invalid yaml content [
`
		configFile := filepath.Join(tmpDir, "invalid-config.yaml")
		err := os.WriteFile(configFile, []byte(invalidContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Load(configFile)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})
}

func TestConfigValidate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "porter-validate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Storage: StorageConfig{
			RootPath: filepath.Join(tmpDir, "test-storage"),
		},
	}

	err = cfg.Validate()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Check if directory was created
	if _, err := os.Stat(cfg.Storage.RootPath); os.IsNotExist(err) {
		t.Error("Storage directory was not created during validation")
	}

	// Check if path was made absolute
	if !filepath.IsAbs(cfg.Storage.RootPath) {
		t.Error("Storage path was not made absolute during validation")
	}
}
