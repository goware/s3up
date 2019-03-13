package main

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type Config struct {
	S3 struct {
		AccessKey string   `toml:"access_key"`
		SecretKey string   `toml:"access_secret_key"`
		Region    string   `toml:"region"`
		Bucket    string   `toml:"bucket"`
		Prefix    string   `toml:"prefix"`
		ACL       string   `toml:"acl"`
		Ignore    []string `toml:"ignore"`
		Source    string   `toml:"source"`
	} `toml:"s3"`
}

func newConfig(file string) (*Config, error) {
	cfg := &Config{}

	if file == "" {
		// empty one..
		return cfg, nil
	}

	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to load config file")
	}

	if _, err := toml.DecodeFile(file, cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse config file")
	}

	return cfg, nil
}
