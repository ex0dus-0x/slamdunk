package slamdunk

import (
    "io"
    "net"
    "log"
    "time"
    "errors"
    "strconv"
    "strings"
    "regexp"
    "net/http"

    "github.com/beevik/etree"
)

const (
    NoBucket = "No bucket found"
    SomeBucket = "Some S3 Bucket"
    NoRegion = "No region found"
)

// Result status for a given target URL. If bucket is nil, means that our current methods 
// could not extrapolate a bucket name.
type ResolverStatus struct {
    // original url
    Url         string

    // resolved bucket name, if found.
    Bucket      string

    // bucket region, if found
    Region      string

    // set if bucket takeover is possible
    Takeover    bool
}

func (r *ResolverStatus) HasBucket() bool {
    return r.Bucket != NoBucket
}

// Given a returned status, create an entry that can be used for display as a row in an ASCII table
func (r *ResolverStatus) GenTableRow() []string {
    return []string{r.Url, r.Bucket, r.Region, strconv.FormatBool(r.Takeover)}
}


// Traverse a CNAME chain to the end and return the resultant URL
func GetCNAME(url string) (string, error) {
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
// 1. Check HTTP GET response for S3 metadata
// 2. Check DNS records for a S3 URL CNAME
// 3. Check if URL itself is a bucket name
// 4. Parse data as XML and check tags for any S3 metadata
func Resolver(url string) (*ResolverStatus, error) {
    // sanity: must not already be an S3 URL
    if strings.Contains(url, "amazonaws.com") {
        return nil, errors.New("Already a S3 URL, no need to resolve further.")
    }

    // get both a qualified URL and normal relative URL
    fullUrl, relativeUrl := GenerateUrlPair(url)

    // default status, nothing found
    status := ResolverStatus {
        Url: relativeUrl,
        Bucket: NoBucket,
        Region: NoRegion,
        Takeover: false,
    }

    // stop hanging on requests that time out
    client := http.Client{
        Timeout: 3 * time.Second,
    }

    // GET request to url and parse out data
    resp, err := client.Get(fullUrl)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    bytedata, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    /////////////////////////////////
    // FIRST CHECK: Request Headers
    /////////////////////////////////

    log.Println("Starting First Check: Request Headers")

    // skip if Google Cloud headers are present
    if resp.Header.Get("X-GUploader-UploadID") != "" {
        return &status, nil
    }

    // check for `Server` header to be AmazonS3, but may be changed by proxy or CDN
    server := resp.Header.Get("Server")
    if server == "AmazonS3" {
        status.Bucket = SomeBucket
    }

    // check if region is set in headers as well
    region := resp.Header.Get("x-amz-bucket-region")
    if region != "" {
        status.Region = region
    }

    ///////////////////////////////
    // SECOND CHECK: CNAME Records
    ///////////////////////////////

    log.Println("Starting Second Check: CNAME Records")

    // check if URL points to a S3 URL in any CNAME records. A bucket may use a CDN that
    // masks the original S3 URL, so this may not return anything even if it is a bucket
    potentialCname, _ := GetCNAME(relativeUrl);
    if strings.Contains(potentialCname, ".amazonaws.com") {

        // s3-<REGION>.amazonaws.com/<BUCKET_NAME>/<OBJECTS>
        expr1 := regexp.MustCompile(`s3-(?P<region>[^.]+).amazonaws.com/(?P<bucket>[^/]+)`)
        expr1Matches := expr1.FindStringSubmatch(potentialCname)
        if len(expr1Matches) != 0 {
            status.Region = expr1Matches[1]
            status.Bucket = expr1Matches[2]
        }

        // <BUCKET_NAME>.s3.<REGION>.amazonaws.com/<OBJECTS>
        expr2 := regexp.MustCompile(`(?P<bucket>[^/]+).s3.(?P<region>[^.]+).amazonaws.com`)
        expr2Matches := expr2.FindStringSubmatch(potentialCname)
        if len(expr2Matches) != 0 {
            status.Region = expr2Matches[2]
            status.Bucket = expr2Matches[1]
        }

        // shouldn't happen, but continue checks if bucket name couldn't be found
        if status.Bucket == NoBucket {
            goto bodyCheck
        }

        // otherwise do a quick takeover check and return
        if strings.Contains(string(bytedata), "NoSuchBucket") {
            status.Takeover = true
        }
        return &status, nil
    }

bodyCheck:

    ///////////////////////////////////
    /// THIRD CHECK: URL AS BUCKET NAME
    ///////////////////////////////////

    log.Println("Starting Third Check: URL as Bucket Name")

    if CheckBucketExists(relativeUrl, status.Region) {
        status.Bucket = relativeUrl
    }

    ///////////////////////////////////
    /// FINAL CHECK: HTTP XML RESPONSE
    ///////////////////////////////////

    // attempt to serialize into proper XML, if not, return
    xml := etree.NewDocument()
    if err := xml.ReadFromBytes(bytedata); err != nil {
        return &status, nil
    }

    // TODO: Check for GCloud error

    // if `Error` root is present, encountered a S3 error page
    if errTag := xml.FindElement("Error"); errTag != nil {

        log.Println("Starting Final Check: Parsing XML Error")

        // get string for Code tag used to indicate error
        code := errTag.SelectElement("Code").Text()

        // NoSuchBucket: bucket deleted, but takeover is possible!
        if code == "NoSuchBucket" {
            status.Bucket = errTag.SelectElement("BucketName").Text()
            status.Takeover = true

        // PermanentRedirect: wrong region, shouldn't be reached
        } else if code == "PermanentRedirect" {
            status.Bucket = errTag.SelectElement("BucketName").Text()

        // AccessDenied | NoSuchKey | etc: bucket exists, can't parse name
        } else {
            status.Bucket = SomeBucket
        }
    }

    // if `ListBucketResult` is present, encountered an open bucket
    if resTag := xml.FindElement("ListBucketResult"); resTag != nil {
        log.Println("Starting Final Check: Parsing XML Front Manner")
        status.Bucket = resTag.SelectElement("Name").Text()
    }
    return &status, nil
}

// Helper that takes a URL in any format and generates a FQDN and a relative URL
func GenerateUrlPair(url string) (string, string) {
    var fullUrl, relativeUrl string

    // if input is relative, construct full and save both
    if !strings.Contains(url, "http") {
        fullUrl = "http://" + url
        relativeUrl = url

    // other way around
    } else {
        fullUrl = url

        // remove prepended protocol
        if strings.Contains(url, "http://") {
            relativeUrl = strings.TrimPrefix(url, "http://")
        } else if strings.Contains(url, "https://") {
            relativeUrl = strings.TrimPrefix(url, "https://")
        }
        relativeUrl = strings.TrimSuffix(relativeUrl, "/")
    }
    return fullUrl, relativeUrl
}
