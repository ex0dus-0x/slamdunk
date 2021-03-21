package slamdunk

import (
    "io"
    "net"
    "errors"
    "strings"
    "net/http"

    "github.com/beevik/etree"
    "github.com/aws/aws-sdk-go/service/s3"
)

// Result status for a given target URL. If bucket is nil, means that our current methods 
// could not extrapolate a bucket name.
type ResolverStatus struct {
    // resolved bucket name, if found. Set as empty string to signify a bucket does exist
    // but could not be resolved
    Bucket *string

    // bucket region, if found
    Region *string

    // set if bucket takeover is possible
    Takeover bool
}


// Traverse a CNAME chain to the end and return the resultant URL
func GetCNAME(url string) (string, error) {

    // remove http protocol when doing cname check
    if strings.Contains(url, "http://") {
        url = strings.TrimPrefix(url, "http://")
    } else if strings.Contains(url, "https://") {
        url = strings.TrimPrefix(url, "https://")
    }

    // do lookup
    cname, err := net.LookupCNAME(url)
    if err != nil {
        return "", errors.New("Domain name doesn't exist")
    }

    // remove trailing dots and compare
    cname = strings.TrimSuffix(cname, ".")
    url = strings.TrimSuffix(url, ".")
    if cname == "" || cname == url {
        return "", errors.New("Domain name is not a CNAME")
    }
    return cname, nil
}

// Given a single URL, run a set of actions against it in order to resolve a bucket name, while also
// attempting to detect if subdomain takeover is possible.
func Resolver(url string) (*ResolverStatus, error) {

    // sanity-check: must not already be an S3 URL
    if strings.Contains(url, "amazonaws.com") {
        return nil, errors.New("Already a S3 URL, no need to resolve further.")
    }

    // default status, nothing found
    status := ResolverStatus {
        Bucket: nil,
        Region: nil,
        Takeover: false,
    }

    // first: check if URL points to a S3 URL in any CNAME records. A bucket may use a CDN that
    // masks the original S3 URL, so this may not return anything even if it is a bucket
    potentialCname, err := GetCNAME(url);
    if err != nil || potentialCname == "" {
        //return nil, err
    }

    // parse CNAME for AWS endpoint
    // endpoint options:
    //  s3-<REGION>.amazonaws.com/<BUCKET_NAME>/<OBJECTS>
    //  <BUCKET_NAME>.s3.<REGION>.amazonaws.com/<OBJECTS>
    if strings.Contains(potentialCname, ".amazonaws.com") {
        // TODO: check regexes
        return &status, nil
    }

    // prepend http protocol to url if not present, since net/http requires so
    if !strings.Contains(url, "http") {
        url = "http://" + url
    }

    // GET request to url and parse out data
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    bytedata, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    // next: check for `Server` header to be AmazonS3, if not, return
    // TODO: more research to see if `Server` header can be manipulated for S3
    server := resp.Header.Get("Server")
    if server == "" || server != "AmazonS3" {
        return &status, nil
    }

    // check if region is set in headers as well
    region := resp.Header.Get("x-amz-bucket-region")
    if region != "" {
        status.Region = &region
    }

    // attempt to serialize into proper XML
    xml := etree.NewDocument()
    if err := xml.ReadFromBytes(bytedata); err != nil {
        goto otherchecks
    }

    // if `Error` root is present, encountered a S3 error page
    if errTag := xml.FindElement("Error"); errTag != nil {
        var bucketName string

        // get string for Code tag used to indicate error
        code := errTag.SelectElement("Code").Text()

        switch code {
            // NoSuchBucket: bucket deleted, but takeover is possible!
            case "NoSuchBucket":
                bucketName = errTag.SelectElement("BucketName").Text()
                status.Takeover = true;

            // PermanentRedirect: wrong region, shouldn't be reached
            case "PermanentRedirect":
                bucketName = errTag.SelectElement("BucketName").Text()

            // AccessDenied | NoSuchKey | etc: bucket exists, can't parse name
            default:
                bucketName = ""
        }

        status.Bucket = &bucketName
        return &status, nil
    }

    // if `ListBucketResult` is present, encountered an open bucket
    if resTag := xml.FindElement("ListBucketResult"); resTag != nil {
        bucketName := resTag.SelectElement("Name").Text()
        status.Bucket = &bucketName
        return &status, nil
    }

otherchecks:

    // if all else fails, check to see if the URL itself is a bucket name
    svc := s3.New(session.New())
    input := &s3.HeadBucketInput{
        Bucket: aws.String(url),
    }

    // check to see if URL bucket exists
    _, err := svc.HeadBucket(input)
    if err != nil {
        return &status, nil
    } else {
        // TODO: parse region?
        status.Bucket = &url
    }
    return &status, nil
}

