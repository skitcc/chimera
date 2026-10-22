package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	bucket := flag.String("bucket", "", "bucket to delete")
	flag.Parse()
	if *bucket == "" {
		log.Fatal("bucket is required")
	}
	endpoint := os.Getenv("S3_ENDPOINT")
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		log.Fatal("S3_ENDPOINT is invalid")
	}
	access := os.Getenv("S3_ACCESS_KEY")
	secret := os.Getenv("S3_SECRET_KEY")
	if access == "" || secret == "" {
		log.Fatal("S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}

	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: u.Scheme == "https",
	})
	if err != nil {
		log.Fatalf("open s3: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	exists, err := client.BucketExists(ctx, *bucket)
	if err != nil {
		log.Fatalf("check bucket: %v", err)
	}
	if !exists {
		return
	}
	for object := range client.ListObjects(ctx, *bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			log.Fatalf("list bucket: %v", object.Err)
		}
		if err := client.RemoveObject(ctx, *bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			log.Fatalf("remove object: %v", err)
		}
	}
	if err := client.RemoveBucket(ctx, *bucket); err != nil {
		log.Fatalf("delete bucket: %v", err)
	}
}
