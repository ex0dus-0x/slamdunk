package slamdunk

import (
    "io"
    "net"
    "errors"
    "strings"
    "regexp"
    "net/http"

    "github.com/beevik/etree"
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
//
// 1. Check DNS records for a S3 URL CNAME
// 2. Check HTTP GET response for S3 metadata
// 3. Parse data as XML and check tags for any S3 metadata
// 4. Check if URL itself is a bucket name
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

    // first: check if URL points to a S3 URL in any CNAME records. A bucket may use a CDN that
    // masks the original S3 URL, so this may not return anything even if it is a bucket
    potentialCname, _ := GetCNAME(url);
    if strings.Contains(potentialCname, ".amazonaws.com") {

        // s3-<REGION>.amazonaws.com/<BUCKET_NAME>/<OBJECTS>
        expr1 := regexp.MustCompile(`s3-(?P<region>[^.]+).amazonaws.com/(?P<bucket>[^/]+)`)
        expr1Matches := expr1.FindStringSubmatch(potentialCname)
        if len(expr1Matches) != 0 {
            status.Region = &expr1Matches[1]
            status.Bucket = &expr1Matches[2]
        }

        // <BUCKET_NAME>.s3.<REGION>.amazonaws.com/<OBJECTS>
        expr2 := regexp.MustCompile(`(?P<bucket>[^/]+).s3.(?P<region>[^.]+).amazonaws.com`)
        expr2Matches := expr2.FindStringSubmatch(potentialCname)
        if len(expr2Matches) != 0 {
            status.Region = &expr2Matches[2]
            status.Bucket = &expr2Matches[1]
        }

        // do a very quick check in the body of data for NoSuchBucket and dip
        if strings.Contains(string(bytedata), "NoSuchBucket") {
            status.Takeover = true
        }
        return &status, nil
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

    // TODO: other XML data that may be returned

otherchecks:

    // check to see if URL itself is a bucket name
    if CheckBucketExists(url) {
        status.Bucket = &url
    }
    return &status, nil
}

