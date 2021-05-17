package slamdunk

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	TempObject = "temp"
)

// Encapsulates all of the actions we can execute against a target bucket.
type PlayBook map[string]Action

// Implementation of a specific heuristic we want to check for against a target.
type Action struct {
	// High-level description of what permission is tested
	Description string

	// equivalent aws CLI command
	Cmd string

	// function called to consume AWS session and wrapped input for testing
	Callback func(s3.S3, string) bool
}

func (a *Action) TableEntry(name string) []string {
	return []string{name, a.Description, "aws s3api " + a.Cmd}
}

func NewPlayBook() PlayBook {
	return map[string]Action{
		"ListObjects": Action{
			Description: "Read and enumerate over objects in bucket.",
			Cmd:         "list-objects --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.ListObjectsInput{
					Bucket:  aws.String(name),
					MaxKeys: aws.Int64(2),
				}
				if _, err := svc.ListObjects(input); err != nil {
					return false
				}
				return true
			},
		},

		"PutObject": Action{
			Description: "Write object to bucket with key.",
			Cmd:         "put-object --bucket <NAME> --key <KEY> --body <FILE>",
			Callback: func(svc s3.S3, name string) bool {

				// get MD5 checksum for empty string
				h := md5.New()
				content := strings.NewReader("")
				content.WriteTo(h)

				resp, _ := svc.PutObjectRequest(&s3.PutObjectInput{
					Bucket: aws.String(name),
					Key:    aws.String(TempObject),
				})

				// configure with MD5 checksum
				md5s := base64.StdEncoding.EncodeToString(h.Sum(nil))
				resp.HTTPRequest.Header.Set("Content-MD5", md5s)

				// create a presigned url to send HTTP request to
				url, err := resp.Presign(time.Minute * 15)
				if err != nil {
					return false
				}

				// send request but with different body to force MD5 check to fail,
				// thus not modifying the actual contents of the bucket
				req, err := http.NewRequest("PUT", url, strings.NewReader("CONTENT"))
				req.Header.Set("Content-MD5", md5s)
				if err != nil {
					return false
				}

				// a successful upload or a failed MD5 checksum check is fine
				finalResp, err := http.DefaultClient.Do(req)
				if finalResp.StatusCode == 200 || finalResp.StatusCode == 400 {
					return true
				} else {
					return false
				}
			},
		},

		"GetBucketAcl": Action{
			Description: "Read bucket's access control list.",
			Cmd:         "get-bucket-acl --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketAclInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketAcl(input); err != nil {
					return false
				}
				return true
			},
		},

		"PutBucketAcl": Action{
			Description: "Write a new access control list for a bucket.",
			Cmd:         "put-bucket-acl --bucket <NAME> --grant-* <KEY_VALUES>",
			Callback: func(svc s3.S3, name string) bool {

				// get MD5 checksum for empty string
				h := md5.New()
				content := strings.NewReader("CONTENT")
				content.WriteTo(h)

				req, _ := svc.PutBucketAclRequest(&s3.PutBucketAclInput{
					Bucket:    aws.String(name),
					GrantRead: aws.String("uri=http://acs.amazonaws.com/groups/global/AllUsers"),
				})

				// configure with invalid MD5 checksum to fail actual modification
				md5s := base64.StdEncoding.EncodeToString(h.Sum(nil))
				req.HTTPRequest.Header.Set("Content-MD5", md5s)

				if err := req.Send(); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketPolicy": Action{
			Description: "Read a bucket's policy.",
			Cmd:         "get-bucket-policy --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketPolicyInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketPolicy(input); err != nil {
					return false
				}
				return true
			},
		},

		"PutBucketPolicy": Action{
			Description: "Write a new policy for the bucket.",
			Cmd:         "put-bucket-policy --bucket <NAME> --policy <FILE>",
			Callback: func(svc s3.S3, name string) bool {
				testPolicy := map[string]interface{}{
					"Version": "2021-01-01",
					"Statement": []map[string]interface{}{
						{
							"Sid":       "AddPerm",
							"Effect":    "Allow",
							"Principal": "*",
							"Action": []string{
								"s3:GetObject",
							},
							"Resource": []string{
								fmt.Sprintf("arn:aws:s3:::%s/*", name),
							},
						},
					},
				}

				policy, _ := json.Marshal(testPolicy)
				input := &s3.PutBucketPolicyInput{
					Bucket: aws.String(name),
					Policy: aws.String(string(policy)),
				}
				if _, err := svc.PutBucketPolicy(input); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketCors": Action{
			Description: "Read a bucket's cross-original resource sharing configuration.",
			Cmd:         "get-bucket-cors --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketCorsInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketCors(input); err != nil {
					return false
				}
				return true
			},
		},

		"PutBucketCors": Action{
			Description: "Read a bucket's cross-original resource sharing configuration.",
			Cmd:         "put-bucket-cors --bucket <NAME> --cors-configuration <FILE>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.PutBucketCorsInput{}
				if _, err := svc.PutBucketCors(input); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketLogging": Action{
			Description: "Gets logging status of bucket and relevant permissions.",
			Cmd:         "get-bucket-logging --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketLoggingInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketLogging(input); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketWebsite": Action{
			Description: "Gets configuration if S3 bucket is configured to serve a site.",
			Cmd:         "get-bucket-website --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketWebsiteInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketWebsite(input); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketVersioning": Action{
			Description: "Get versioning status of the bucket.",
			Cmd:         "get-bucket-versioning --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketVersioningInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketVersioning(input); err != nil {
					return false
				}
				return true
			},
		},

		"GetBucketEncryption": Action{
			Description: "Get encryption configuration of bucket, if any.",
			Cmd:         "get-bucket-encryption --bucket <NAME>",
			Callback: func(svc s3.S3, name string) bool {
				input := &s3.GetBucketEncryptionInput{
					Bucket: aws.String(name),
				}
				if _, err := svc.GetBucketEncryption(input); err != nil {
					return false
				}
				return true
			},
		},

		// GetBucketPublicAccessBlock
	}
}
