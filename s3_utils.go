package slamdunk

import (
    "os"
    "fmt"
    "log"
    "os/user"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/service/s3"
)

func GetRegions() []string {
    return []string{
        "us-east-2",
        "us-east-1",
        "us-west-1",
        "us-west-2",
        /*
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
        */
    }
}

// Helper used to check if the current user is authenticated, as some permissions are configured
// to work only if its by an authenticated user. We won't parse the credentials if it exists, as the
// S3 SDK should be doing that for us.
func IsAuthenticated() bool {
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
func HeadBucket(target string, region string) bool {
    // configure session to work in specific region
    sess, _ := session.NewSession(&aws.Config{
        Region: aws.String(region)},
    )
    svc := s3.New(sess)

    // create new wrapped input for the specific operation
    input := &s3.HeadBucketInput{
        Bucket: aws.String(target),
    }

    // check to see if URL bucket exists
    _, err := svc.HeadBucket(input)
    if err != nil {

        // if AccessDenied or InvalidKey, the bucket exists but may lack permissiosn
        // to access it, requiring further investigation (TODO: ErrCodeNoSuchUpload)
        if aerr, ok := err.(awserr.Error); ok {
            errMsg := aerr.Code()

            // AccessDenied means bucket exists, unless in China region, which reports that for all
            if (errMsg == "Forbidden") && (region != "cn-north-1") && (region != "cn-northwest-1") {
                return true

            // InvalidKey means bucket exists but points to a deleted object
            } else if (errMsg == s3.ErrCodeNoSuchKey) {
                return true

            // missing* may be a s3 specific error, possible latency issues
            } else if (errMsg == "MissingEndpoint") || (errMsg == "MissingRegion") {
                log.Println("May be encountering a rate limit/timeout.")
                return false

            // anything else, such as InvalidBucket
            } else {
                return false
            }
        }
    }
    return true
}

// Helper that checks if a bucket exists within a region, returning the status and region name. 
// If no region is specified, the supported list of AWS regions will be checked and returned.
func CheckBucketExists(target string, region string) (bool, string) {
    if region == NoRegion || region == "" {
        for _, r := range GetRegions() {
            if val := HeadBucket(target, r); val {
                return true, r
            }
        }
        return false, ""
    }
    return HeadBucket(target, region), region
}
