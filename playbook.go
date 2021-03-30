package slamdunk

import (
    "bytes"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/s3"
)

const (
    TempObject = "Test upload for testing PutObject permissions"
)

// Encapsulates all of the actions we can execute against a target bucket.
type PlayBook map[string]Action

// Implementation of a specific heuristic we want to check for against a target.
type Action struct {
    // High-level description of what permission is tested
    Description    string

    // equivalent aws CLI command
    Cmd            string

    // function called to consume AWS session and wrapped input for testing
    Callback       func(s3.S3, string) bool
}

func (a *Action) TableEntry(name string) []string {
    return []string{name, a.Description, "aws s3api " + a.Cmd}
}

func NewPlayBook() PlayBook {
    return map[string]Action {
        "ListObjects": Action {
            Description: "Read and enumerate over objects in bucket.",
            Cmd: "list-objects --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.ListObjectsInput{
                    Bucket:  aws.String(name),
                    MaxKeys: aws.Int64(2),
                }
                _, err := svc.ListObjects(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "PutObject": Action {
            Description: "Write object to bucket with key.",
            Cmd: "put-object --bucket <NAME> --key <KEY> --body <FILE>",
            Callback: func(svc s3.S3, name string) bool {
                reader := bytes.NewReader([]byte(TempObject))
                input := &s3.PutObjectInput{
                    Body:   aws.ReadSeekCloser(reader),
                    Bucket: aws.String(name),
                    Key:    aws.String(TempObject),
                }
                _, err := svc.PutObject(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "GetBucketAcl": Action {
            Description: "Read bucket's access control list.",
            Cmd: "get-bucket-acl --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.GetBucketAclInput{
                    Bucket: aws.String(name),
                }
                _, err := svc.GetBucketAcl(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "PutBucketAcl": Action {
            Description: "Write a new access control list for a bucket.",
            Cmd: "put-bucket-acl --bucket <NAME> --grant-full-control emailaddress=<EMAIL>",
            Callback: func(svc s3.S3, name string) bool {
                return false
            },
        },

        "GetBucketPolicy": Action {
            Description: "Read a bucket's policy.",
            Cmd: "get-bucket-policy --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.GetBucketPolicyInput{
                    Bucket: aws.String(name),
                }
                _, err := svc.GetBucketPolicy(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "PutBucketPolicy": Action {
            Description: "Write a new policy for the bucket.",
            Cmd: "put-bucket-acl --bucket <NAME> --policy <FILE>",
            Callback: func(svc s3.S3, name string) bool {
                return false
            },
        },

        "GetBucketCors": Action {
            Description: "Read a bucket's cross-original resource sharing configuration.",
            Cmd: "get-bucket-cors --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.GetBucketCorsInput{
                    Bucket: aws.String(name),
                }
                _, err := svc.GetBucketCors(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "PutBucketCors": Action {
            Description: "Read a bucket's cross-original resource sharing configuration.",
            Cmd: "put-bucket-cors --bucket <NAME> --cors-configuration <FILE>",
            Callback: func(svc s3.S3, name string) bool {
                return false
            },
        },

        "GetBucketLogging": Action {
            Description: "Gets logging status of bucket and relevant permissions.",
            Cmd: "get-bucket-logging --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.GetBucketLoggingInput{
                    Bucket: aws.String(name),
                }
                _, err := svc.GetBucketLogging(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        "GetBucketWebsite": Action {
            Description: "Gets configuration if S3 bucket is configured to serve a site.",
            Cmd: "get-bucket-website --bucket <NAME>",
            Callback: func(svc s3.S3, name string) bool {
                input := &s3.GetBucketWebsiteInput{
                    Bucket: aws.String(name),
                }
                _, err := svc.GetBucketWebsite(input)
                if err != nil {
                    return false
                }
                return true
            },
        },

        // GetBucketPublicAccessBlock
        // GetBucketVersioning
        // GetEncryptionConfiguration
    }
}
