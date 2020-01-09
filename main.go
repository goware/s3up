package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	flags = flag.NewFlagSet("s3up", flag.ExitOnError)

	versionFlag    = flags.Bool("version", false, "print the version and exit")
	configFlag     = flags.String("config", "", "config file")
	accessKeyFlag  = flags.String("access-key", "", "s3 access key")
	secretKeyFlag  = flags.String("secret-key", "", "s3 secret key")
	regionFlag     = flags.String("region", "", "s3 region")
	bucketFlag     = flags.String("bucket", "", "s3 bucket")
	prefixFlag     = flags.String("prefix", "", "s3 path prefix")
	sourcePathFlag = flags.String("source", "", "local source path to upload")
	dryrunFlag     = flags.Bool("dryrun", false,
		"Dryrun, dont upload anything, just list files to upload")
	confirmFlag = flags.Bool("confirm", false,
		"Confirm final settings with user before triggering upload")
	parallelFlag   = flags.Int("parallel", 10, "Number of parallel uploads (default=10)")
	hashPrefixFlag = flags.Bool("auto-content-hash-prefix", false,
		"Compute the md5 hash of a file's contents and use it as the path prefix")
	hashPrefixBytesFlag = flags.Uint("content-hash-bytes", 6,
		"Number of bytes of md5 hash to use in the path prefix. Max 16. 6 and 12 best (base64 encoding)")
	syncFlag = flags.Bool("sync", false,
		"Only upload files that are new or have changed")
	listFlag = flags.Bool("list", false,
		"List files data to be processed (outputs in JSON format)")
	manifestFlag = flags.String("manifest", "", "path to write a manifest file")
	cacheTTLFlag = flags.Int("cache-ttl", 0, "TTL for cache control headers (in seconds)")
)

const VERSION = "0.3.1"

func main() {
	flags.Parse(os.Args[1:])

	if len(os.Args) == 1 {
		flags.Usage()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	cfg, err := newConfig(*configFlag)
	if err != nil {
		log.Fatal(err)
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
	if *cacheTTLFlag > 0 {
		cfg.S3.ExpiresAfterSeconds = *cacheTTLFlag
		cfg.S3.CacheControl = fmt.Sprintf("public, max-age=%d", *cacheTTLFlag)
	}

	if cfg.S3.Source == "" {
		log.Fatal("invalid source path")
	}

	s3up, err := NewS3Upload(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// just list files, it is like dry-run but with output as JSON array
	if *listFlag {
		fileDataList, err := s3up.sourceFiles()
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "    ")
		if err := enc.Encode(fileDataList); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	if err := s3up.Connect(); err != nil {
		log.Fatal(err)
	}

	if *confirmFlag && !confirm(cfg) {
		os.Exit(0)
	}

	// run actual upload
	n, err := s3up.Upload(*parallelFlag, *dryrunFlag)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nDone! uploaded %d files.\n", n)
}

func confirm(cfg *Config) bool {
	fmt.Println("s3up config:")
	fmt.Println("------------")
	fmt.Println("BUCKET      :  ", cfg.S3.Bucket)
	fmt.Println("PREFIX      :  ", cfg.S3.Prefix)
	fmt.Println("SOURCE      :  ", cfg.S3.Source)
	fmt.Println("ACL         :  ", cfg.S3.ACL)
	fmt.Println("IGNORE      :  ", cfg.S3.Ignore)
	fmt.Println("HASH PREFIX :  ", *hashPrefixFlag)
	fmt.Println("SYNC        :  ", *syncFlag)
	fmt.Println("")
	fmt.Printf("upload? (y/n): ")

	r := bufio.NewReader(os.Stdin)
	char, _, err := r.ReadRune()

	if err != nil {
		log.Fatal(err)
	}

	switch char {
	case 'Y':
		return true
	case 'y':
		return true
	}

	return false
}
