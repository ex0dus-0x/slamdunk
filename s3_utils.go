package slamdunk

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

func CheckBucketExists(target string) bool {
    // if all else fails, check to see if the URL itself is a bucket name
    svc := s3.New(session.New())
    input := &s3.HeadBucketInput{
        Bucket: aws.String(target),
    }

    // check to see if URL bucket exists
    _, err := svc.HeadBucket(input)
    if err != nil {
        return false
    }
    return true
}
