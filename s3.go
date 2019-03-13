package main

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	zglob "github.com/mattn/go-zglob"
)

type S3Upload struct {
	Config *Config
	Conn   *s3.S3
}

func NewS3Upload(cfg *Config) (*S3Upload, error) {
	var err error
	s3c := &S3Upload{Config: cfg}
	s3c.Conn, err = s3c.newSession()
	if err != nil {
		return nil, err
	}
	return s3c, nil
}

func (s *S3Upload) newSession() (*s3.S3, error) {
	cfg := s.Config

	sess, err := session.NewSession(aws.NewConfig())
	if err != nil {
		return nil, err
	}
	sess.Config.WithCredentials(credentials.NewStaticCredentials(cfg.S3.AccessKey, cfg.S3.SecretKey, ""))

	region := cfg.S3.Region
	if region == "" {
		region, err = s3manager.GetBucketRegion(context.Background(), sess, cfg.S3.Bucket, "us-west-2")
		if err != nil {
			return nil, err
		}
	}

	if region == "" {
		return nil, errors.New("unknown region")
	}
	sess.Config.WithRegion(region)

	return s3.New(sess), nil
}

func (s *S3Upload) isUploadableFile(path string) (bool, error) {
	for _, pat := range s.Config.S3.Ignore {
		match, err := zglob.Match(pat, path)
		if err != nil {
			return false, err
		}
		if match {
			return false, nil
		}
	}
	return true, nil
}

func (s *S3Upload) sourceFiles() ([]string, error) {
	var files []string
	source := s.Config.S3.Source

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		// Skip ignored files
		ok, err := s.isUploadableFile(path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		// Skip if path is directory
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		// Add to the list of files to upload
		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func (s *S3Upload) Upload() (int, error) {
	files, err := s.sourceFiles()
	if err != nil {
		return 0, err
	}

	// upload to S3
	num := 0
	s3c := s.Conn

	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return num, err
		}

		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		destPath := filepath.Join("/", s.Config.S3.Prefix, path)

		fmt.Printf("uploading %s ...\n", destPath)

		acl := s.Config.S3.ACL
		if acl == "" {
			acl = "private"
		}

		obj := &s3.PutObjectInput{
			Bucket:      aws.String(s.Config.S3.Bucket),
			Key:         aws.String(destPath),
			ACL:         aws.String(acl),
			ContentType: aws.String(mimeType),
			Body:        file,
		}

		req, _ := s3c.PutObjectRequest(obj)
		if err := req.Send(); err != nil {
			return num, err
		}
		num += 1
	}

	return num, nil
}
