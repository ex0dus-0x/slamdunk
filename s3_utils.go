package slamdunk

import (
    "os"
    "fmt"
    "os/user"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)


// Helper used to check if the current user is authenticated, as some permissions are configured
// to work only if its by an authenticated user. We won't parse the credentials if it exists, as the
// S3 SDK should be doing that for us.
func AWSIsAuthenticated() bool {
    // resolve standard path to where credentials should be
    user, _ := user.Current()
    dir := user.HomeDir
    path := fmt.Sprintf("%s/.aws/credentials", dir)

    // filepath check
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return false
    }
    return true
}


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
