package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Auth    AuthConfig    `yaml:"auth"`
	Logging LoggingConfig `yaml:"logging"`
}

type ServerConfig struct {
	Address string    `yaml:"address"`
	TLS     TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type StorageConfig struct {
	RootPath string `yaml:"root_path"`
	MaxSize  int64  `yaml:"max_size_bytes"`
}

type AuthConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address: ":9000",
			TLS: TLSConfig{
				Enabled: false,
			},
		},
		Storage: StorageConfig{
			RootPath: "./data",
			MaxSize:  100 * 1024 * 1024 * 1024, // 100GB
		},
		Auth: AuthConfig{
			AccessKey: "porterfs",
			SecretKey: "porterfs",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func (c *Config) Validate() error {
	if c.Storage.RootPath == "" {
		c.Storage.RootPath = "./data"
	}

	if err := os.MkdirAll(c.Storage.RootPath, 0755); err != nil {
		return err
	}

	absPath, err := filepath.Abs(c.Storage.RootPath)
	if err != nil {
		return err
	}
	c.Storage.RootPath = absPath

	return nil
}
