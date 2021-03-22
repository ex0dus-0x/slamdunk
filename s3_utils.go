package slamdunk

import (
    "os"
    "fmt"
    "os/user"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/service/s3"
)


// Helper to retrieve identifiers for all bucket regions
func GetBucketRegions() []string {
    return []string{
        "us-east-1", // default
        "us-east-2",
        "us-west-1",
        "us-west-2",
        "ap-south-1",
        "ap-northeast-3",
        "ap-northeast-2",
        "ap-southeast-1",
        "ap-southeast-2",
        "ap-northeast-1",
        "ca-central-1",
        "cn-north-1",
        "cn-northwest-1",
        "eu-central-1",
        "eu-west-1",
        "eu-west-2",
        "eu-west-3",
        "eu-north-1",
        "sa-east-1",
    }
}


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


// Does a single `HeadBucket` operation against a target bucket given a name and region.
func HeadBucketWrap(target string, region string) bool {
    // configure session to work in specific region
    sess, _ := session.NewSession(&aws.Config{
        Region: aws.String(region)},
    )
    svc := s3.New(sess)
    input := &s3.HeadBucketInput{
        Bucket: aws.String(target),
    }

    // check to see if URL bucket exists
    _, err := svc.HeadBucket(input)
    if err != nil {

        // if AccessDenied or InvalidKey, the bucket exists but may lack permissiosn
        // to access it, requiring further investigation (TODO: ErrCodeNoSuchUpload)
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            case "Forbidden", s3.ErrCodeNoSuchKey:
                return true
            case "MissingEndpoint":
                fmt.Println("May be encountering a rate limit/timeout.")
                fallthrough
            case s3.ErrCodeNoSuchBucket: // NotFound
                fallthrough
            default:
                return false
            }
        }
    }
    return true
}


// Helper that iterates through each region to check if the bucket exists using HeadBucket.
// Returns bool and also region if bucket is found.
func CheckBucketExists(target string, region string) (bool, string) {

    // check if specific region is found to check against, is a lot faster
    // should return true, ie if resolver figured out a region beforehand for you
    if region != NoRegion {
        return HeadBucketWrap(target, region), region
    }

    // otherwise enumerate over each region and check
    for _, iterRegion := range GetBucketRegions() {
        if HeadBucketWrap(target, iterRegion) == true {
            return true, iterRegion
        }
    }
    return false, ""
}
