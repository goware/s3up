package main

import (
	"mime"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type Config struct {
	S3 struct {
		AccessKey           string     `toml:"access_key"`
		SecretKey           string     `toml:"access_secret_key"`
		Region              string     `toml:"region"`
		Bucket              string     `toml:"bucket"`
		Prefix              string     `toml:"prefix"`
		ACL                 string     `toml:"acl"`
		CacheControl        string     `toml:"cache_control"`
		ExpiresAfterSeconds int        `toml:"expires_after_seconds"`
		MimeTypes           []MimeType `toml:"mime_types"`
		Ignore              []string   `toml:"ignore"`
		Source              string     `toml:"source"`
	} `toml:"s3"`
}

type MimeType struct {
	Extension string `toml:"ext"`
	Type      string `toml:"type"`
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

	for _, newMime := range cfg.S3.MimeTypes {
		if err := mime.AddExtensionType(newMime.Extension, newMime.Type); err != nil {
			return nil, errors.Wrap(err, "failed to parse config file")
		}
	}

	return cfg, nil
}
