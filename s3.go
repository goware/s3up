package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/mattn/go-zglob"
)

var expiry *time.Time

type S3Upload struct {
	Config      *Config
	Conn        *s3.S3
	SourcePath  string
	expires     *time.Time
	prefixBytes int
}

// FileData contains data of file to be uploaded
type FileData struct {
	origPath   string `json:"path"`
	Path       string 
	Size       int64  `json:"size"`
	FilePrefix string `json:"prefix"`
	MD5Hash    string `json:"-"`
}

func NewS3Upload(cfg *Config) (*S3Upload, error) {
	var err error
	s3c := &S3Upload{Config: cfg}
	s3c.SourcePath, err = filepath.Abs(cfg.S3.Source)
	if err != nil {
		return nil, err
	}
	if cfg.S3.ExpiresAfterSeconds != 0 {
		t := time.Now().UTC().Add(time.Second * time.Duration(cfg.S3.ExpiresAfterSeconds))

		s3c.expires = &t
	}

	if hashPrefixBytesFlag != nil && prefixFlag != nil {
		s3c.prefixBytes = int(*hashPrefixBytesFlag)
		if s3c.prefixBytes > 16 {
			s3c.prefixBytes = 16
		}
	}

	return s3c, nil
}

// Connect opens connection to AWS and sets up session
func (s *S3Upload) Connect() error {
	var err error
	s.Conn, err = s.newSession()
	if err != nil {
		return err
	}

	return nil
}

func (s *S3Upload) newSession() (*s3.S3, error) {
	cfg := s.Config

	awsConfig := &aws.Config{}

	sess, err := session.NewSession(awsConfig)
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

func (s *S3Upload) sourceFiles() ([]*FileData, error) {
	var files []*FileData
	source := s.SourcePath

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		// Skip ignored files
		cpath := strings.TrimPrefix(path, s.SourcePath)
		if cpath == "" {
			return nil
		}
		cpath = cpath[1:]
		ok, err := s.isUploadableFile(cpath)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Skip if path is directory
		fileInfo, err := file.Stat()
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			return nil
		}

		// calculate md5-hash of file
		h := md5.New()
		if _, err := io.Copy(h, file); err != nil {
			return err
		}
		md5Hash := h.Sum(nil)

		// add md5 hash as prefix if required
		hashPrefix := ""
		if s.prefixBytes > 0 {
			hashPrefix = base64.URLEncoding.EncodeToString(md5Hash[:s.prefixBytes])
		}

		destPath := filepath.Join("/", hashPrefix, s.Config.S3.Prefix, cpath)

		// Add to the list of files to upload
		files = append(
			files,
			&FileData{
				origPath:   path,
				Path:       destPath,
				Size:       fileInfo.Size(),
				MD5Hash:    fmt.Sprintf("%x", md5Hash),
				FilePrefix: hashPrefix,
			},
		)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func (s *S3Upload) uploadFile(fileData *FileData, dryrun bool) (int, error) {
	s3c := s.Conn

	file, err := os.Open(fileData.origPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// check if object exists if specified in flags
	if *syncFlag {
		headOutput, err := s3c.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(s.Config.S3.Bucket),
			Key:    aws.String(fileData.Path),
		})

		if err == nil {
			// file exists on S3, check if we need to proceed with upload
			if *hashPrefixFlag {
				// don't upload if md5-hash prefix was used and it is already on S3
				fmt.Printf("File %s is already on S3\n", fileData.Path)
				return 0, nil
			} else {
				// md5-hash wasn't used to upload, lets compare md5-hashes on S3 and local file system
				if headOutput != nil &&
					headOutput.ETag != nil && strings.Trim(*headOutput.ETag, "\"") == fileData.MD5Hash {
					fmt.Printf("File %s hasn't been changed (copy on S3 has the same md5-hash in ETag)\n",
						fileData.Path)
					return 0, nil
				}
			}
		} else {
			// check error, return if it is not 404
			if reqErr, ok := err.(awserr.RequestFailure); !ok {
				return 0, err
			} else if reqErr.StatusCode() != http.StatusNotFound {
				return 0, err
			}
		}
	}

	if dryrun {
		fmt.Printf("[DRYRUN] uploading %s ...\n", fileData.Path)
		return 0, nil
	}

	fmt.Printf("uploading %s ...\n", fileData.Path)

	mimeType := mime.TypeByExtension(filepath.Ext(fileData.origPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	acl := s.Config.S3.ACL
	if acl == "" {
		acl = "private"
	}

	obj := &s3.PutObjectInput{
		Bucket:      aws.String(s.Config.S3.Bucket),
		Key:         aws.String(fileData.Path),
		ACL:         aws.String(acl),
		ContentType: aws.String(mimeType),
		Body:        file,
		Expires:     s.expires,
	}

	if s.Config.S3.CacheControl != "" {
		obj.CacheControl = aws.String(s.Config.S3.CacheControl)
	}

	req, _ := s3c.PutObjectRequest(obj)
	if err := req.Send(); err != nil {
		return 0, err
	}

	return 1, nil
}

func (s *S3Upload) Upload(parallel int, dryrun bool) (uint64, error) {
	// get list of files with data
	files, err := s.sourceFiles()
	if err != nil {
		return 0, err
	}

	fch := make(chan *FileData, len(files))
	for _, fileData := range files {
		fch <- fileData
	}
	close(fch)

	var num uint64

	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for fileData := range fch {
				numRetries := 30
			RETRY:
				n, err := s.uploadFile(fileData, dryrun)
				if err != nil {
					_, ok := err.(awserr.Error)
					if ok {
						numRetries -= 1
						if numRetries > 0 {
							// retry in 1 second
							fmt.Printf("failed to upload %s, retrying in 1 second ...\n", fileData.origPath)
							time.Sleep(1 * time.Second)
							goto RETRY
						} else {
							panic(err)
						}
					} else {
						panic(fmt.Sprintf("unknown error! %v", err))
					}
				}
				atomic.AddUint64(&num, uint64(n))
			}
		}(i)
	}

	wg.Wait()
	return num, nil
}
