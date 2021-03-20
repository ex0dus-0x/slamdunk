package slamdunk

import (
    "io"
    "fmt"
    "errors"
    "strings"
    "net/http"
    "encoding/xml"
)

// Result status for a given target URL. If bucket is nil, means that our current methods 
// could not extrapolate a bucket name.
type ResolverStatus struct {
    // original target url
    Url string

    // resolved bucket name, if found
    Bucket *string

    // bucket region, if found
    Region *string

    // set if URL resolves to bucket. If `Bucket` is nil we can still report to user that
    // a S3 bucket does exist
    BucketPresent bool

    // set if bucket takeover is possible
    Takeover bool
}

// Once done resolving, use to output data back to user
// TODO: output format
func (r *ResolverStatus) Output() {
    fmt.Printf("URL `%s` -> ", r.Url)
    if r.Bucket != nil {
        fmt.Printf("%s ", *r.Bucket)
        if r.Takeover {
            fmt.Printf("(Vulnerable to takeover)\n")
        } else {
            fmt.Printf("\n")
        }
    } else {
        fmt.Printf("UNKNOWN!\n")
    }
    fmt.Printf("\n")
}

// If the URL redirects to an S3 Error page, this is the XML structure we use.
// We'll check the `Code` attribute to see which specific errors we can use to
// extrapolate a name out, and check if takeover is possible.
type S3Error struct {
	XMLName    xml.Name `xml:"Error"`
	Text       string   `xml:",chardata"`
	Code       string   `xml:"Code"`
	Message    string   `xml:"Message"`
	BucketName string   `xml:"BucketName"`
	RequestId  string   `xml:"RequestId"`
	HostId     string   `xml:"HostId"`
}

// If the URL redirects to a ListBucketResult page, this is the XML structure we use.
// The name should be included, as well as visible objects, which means ListObjects is allowed.
type S3Results struct {
	XMLName     xml.Name `xml:"ListBucketResult"`
	Text        string   `xml:",chardata"`
	Name        string   `xml:"Name"`
	Prefix      string   `xml:"Prefix"`
	Marker      string   `xml:"Marker"`
	MaxKeys     int      `xml:"MaxKeys"`
	IsTruncated bool     `xml:"IsTruncated"`
	Contents    []struct {
		Text         string `xml:",chardata"`
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int    `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	} `xml:"Contents"`
}

// Given a single URL, run a set of actions against it in order to resolve a bucket name, while also
// attempting to detect if subdomain takeover is possible.
func Resolver(url string) (*ResolverStatus, error) {

    // sanity-check: must not already be an S3 URL
    if strings.Contains(url, "amazonaws.com") {
        return nil, errors.New("Already a S3 URL, no need to resolve further.")
    }

    // prepend http protocol to url if not present
    if !strings.Contains(url, "http") {
        url = "http://" + url
    }

    // GET request to url to parse out data
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // parse body as data
    bytedata, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    // first attempt: serialize into error XML
    var bucketErr S3Error
    stat := xml.Unmarshal(bytedata, &err)

    // success: means we encountered an S3 error with the URL, so parse info and return status
    if stat == nil {

        // NoSuchBucket: takeover is possible!
        if bucketErr.Code == "NoSuchBucket" {
            return &ResolverStatus {
                Url: url,
                Bucket: &bucketErr.BucketName,
                Region: nil,
                BucketPresent: true,
                Takeover: true,
            }, nil


        // PermanentRedirect: wrong region, shouldn't be reached
        } else if bucketErr.Code == "PermanentRedirect" {
            return &ResolverStatus {
                Url: url,
                Bucket: &bucketErr.BucketName,
                Region: nil,
                BucketPresent: true,
                Takeover: false,
            }, nil

        // AccessDenied: bucket exists, can't parse name
        } else if bucketErr.Code == "AccessDenied" || bucketErr.Code == "NoSuchKey" {
            return &ResolverStatus {
                Url: url,
                Bucket: nil,
                Region: nil,
                BucketPresent: true,
                Takeover: false,
            }, nil
        }

        // TODO: other errors that may occur
    }

    // second attempt: serialize into XML entries
    var results S3Results
    stat = xml.Unmarshal(bytedata, &results)

    // success: parse out the bucket name, takeover isn't possible
    if stat == nil {
        return &ResolverStatus {
            Url: url,
            Bucket: &results.Name,
            Takeover: false,
        }, nil
    }

    // if parsing the page yields nothing, check DNS CNAME records to see if it points to a S3 URL

    // if all else fails, check to see if the URL itself is a bucket name

    // if everything absolutely fails, return failed status
    return &ResolverStatus {
        Url: url,
        Bucket: nil,
        Takeover: false,
    }, nil
}

