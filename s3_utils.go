package slamdunk

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sts"
	"log"
	"os"
	"os/user"
)

// Determine the bucket region using a default regionHint of `us-east-1`
func GetRegion(bucket string) (string, error) {
	sess := session.Must(session.NewSession())
	region, err := s3manager.GetBucketRegion(aws.BackgroundContext(), sess, bucket, "us-east-1")
	if err != nil {
		return "", err
	}
	return region, nil
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
	log.Println("Checking credentials path exists")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// Get the current IAM user's identity metadata, and return ARN
func GetIAMUserARN(profile string) (string, error) {
	sess, _ := session.NewSessionWithOptions(session.Options{
		Profile: profile,
	})

	log.Println("Running GetCallerIdentity to parse ARN")
	svc := sts.New(sess)
	input := &sts.GetCallerIdentityInput{}
	result, err := svc.GetCallerIdentity(input)
	if err != nil {
		return "", err
	}
	return *result.Arn, nil
}

// Given a profile, parse out all accessible buckets, if possible
func ListBuckets(profile string) (*[]string, error) {
	sess, _ := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String("us-east-2"), // TODO: figure out beforehand
		},
	})
	svc := s3.New(sess)

	// retrieve buckets and error handle
	log.Println("Running ListBucket")
	input := &s3.ListBucketsInput{}
	result, err := svc.ListBuckets(input)
	if err != nil {
		return nil, err
	}

	// iterate over results and save to list to return
	log.Println("Parsing out bucket names to return")
	buckets := []string{}
	for _, entry := range result.Buckets {
		buckets = append(buckets, *entry.Name)
	}
	return &buckets, nil
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
	log.Println("Running HeadBucket")
	_, err := svc.HeadBucket(input)
	if err != nil {

		// if AccessDenied or InvalidKey, the bucket exists but may lack permissiosn
		// to access it, requiring further investigation (TODO: ErrCodeNoSuchUpload)
		if aerr, ok := err.(awserr.Error); ok {
			errMsg := aerr.Code()

			log.Println("Parsing error message to properly return response")

			// AccessDenied means bucket exists, unless in China region, which reports that for all
			if (errMsg == "Forbidden") && (region != "cn-north-1") && (region != "cn-northwest-1") {
				return true

				// InvalidKey means bucket exists but points to a deleted object
			} else if errMsg == s3.ErrCodeNoSuchKey {
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
	// if no region specified, try to figure it out and return
	if region == NoRegion || region == "" {
		log.Println("Attempting to figure out region for bucket")
		newRegion, err := GetRegion(target)
		if err != nil {
			return false, ""
		}
		return true, newRegion
	}
	return HeadBucket(target, region), region
}
