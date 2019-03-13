package main

import (
	"bufio"
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
	confirmFlag    = flags.Bool("confirm", false, "Confirm final settings with user before triggering upload")
	parallelFlag   = flags.Int("parallel", 10, "Number of parallel uploads (default=10)")
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

	if *confirmFlag {
		if !confirm(cfg) {
			os.Exit(0)
		}
	}

	s3up, err := NewS3Upload(cfg)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	n, err := s3up.Upload(*parallelFlag, *dryrunFlag)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Printf("\nDone! uploaded %d files.\n", n)
}

func confirm(cfg *Config) bool {
	fmt.Println("s3up config:")
	fmt.Println("------------")
	fmt.Println("BUCKET     :  ", cfg.S3.Bucket)
	fmt.Println("PREFIX     :  ", cfg.S3.Prefix)
	fmt.Println("SOURCE     :  ", cfg.S3.Source)
	fmt.Println("ACL        :  ", cfg.S3.ACL)
	fmt.Println("IGNORE     :  ", cfg.S3.Ignore)
	fmt.Println("")
	fmt.Printf("upload? (y/n): ")

	r := bufio.NewReader(os.Stdin)
	char, _, err := r.ReadRune()

	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	switch char {
	case 'Y':
		return true
		break
	case 'y':
		return true
		break
	}

	return false
}
