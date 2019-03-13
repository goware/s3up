package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	flags = flag.NewFlagSet("s3up", flag.ExitOnError)

	configFlag     = flags.String("config", "", "config file")
	accessKeyFlag  = flags.String("access_key", "", "s3 access key")
	secretKeyFlag  = flags.String("secret_key", "", "s3 secret key")
	regionFlag     = flags.String("region", "", "s3 region")
	bucketFlag     = flags.String("bucket", "", "s3 bucket")
	prefixFlag     = flags.String("prefix", "", "s3 path prefix")
	sourcePathFlag = flags.String("source", "", "local source path to upload")
	dryrunFlag     = flags.Bool("dryrun", false, "Dryrun, dont upload anything, just list files to upload")
)

func main() {
	flags.Parse(os.Args[1:])

	cfg, err := newConfig(*configFlag)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// override from file configs if specific flags provided
	if *accessKeyFlag != "" {
		cfg.S3.AccessKey = *accessKeyFlag
	}
	if *secretKeyFlag != "" {
		cfg.S3.SecretKey = *secretKeyFlag
	}
	if *regionFlag != "" {
		cfg.S3.Region = *regionFlag
	}
	if *bucketFlag != "" {
		cfg.S3.Bucket = *bucketFlag
	}
	if *prefixFlag != "" {
		cfg.S3.Prefix = *prefixFlag
	}
	if *sourcePathFlag != "" {
		cfg.S3.Source = *sourcePathFlag
	}

	if cfg.S3.Source == "" {
		log.Fatal("invalid source path")
		os.Exit(1)
	}

	s3up, err := NewS3Upload(cfg)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	n, err := s3up.Upload(*dryrunFlag)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Printf("\nDone! uploaded %d files.\n", n)
}
