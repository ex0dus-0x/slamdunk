package slamdunk

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

const (
	NoBucket   = "No bucket found"
	SomeBucket = "Some S3 Bucket"
	NoRegion   = "No region found"
)

// Result status for a given target URL
type ResolverStatus struct {
	// original url
	Url string

	// resolved bucket name, if found.
	Bucket string

	// bucket region, if found
	Region string

	// set if bucket takeover is possible
	Takeover bool
}

// Given a returned status, create an entry that can be used for display as a row in an ASCII table
func (r *ResolverStatus) Row() []string {
	return []string{r.Url, r.Bucket, r.Region, strconv.FormatBool(r.Takeover)}
}

type Resolver struct {
	// buckets successfully parsed out
	Buckets []ResolverStatus

	// number of URLs successfully processed
	UrlsProcessed int

	// number of URLS failed to process (ie timeout)
	UrlsFailed int

	// S3 endpoints identified, even if name can't be found
	Endpoints int

	// how many endpoints can be taken over
	TakeoverPossible int
}

func NewResolver() *Resolver {
	return &Resolver{
		Buckets:          []ResolverStatus{},
		UrlsProcessed:    0,
		UrlsFailed:       0,
		Endpoints:        0,
		TakeoverPossible: 0,
	}
}

// Given a single URL, run a set of actions against it in order to resolve a bucket name, while also
// attempting to detect if subdomain takeover is possible.
//
// 1. Check HTTP GET response for S3 metadata
// 2. Check DNS records for a S3 URL CNAME
// 3. Check if URL itself is a bucket name
// 4. Parse data as XML and check tags for any S3 metadata
func (r *Resolver) Resolve(url string) error {
	log.Println("Sanity check if already an AWS URL")
	if strings.Contains(url, "amazonaws.com") {
		r.UrlsFailed += 1
		return errors.New("Already a S3 URL, no need to resolve further.")
	}

	// get both a qualified URL and normal relative URL
	log.Println("Creating relative and full URLs for HTTP and DNS.")
	fullUrl, relativeUrl := GenerateUrlPair(url)

	// default status, nothing found
	status := ResolverStatus{
		Url:      relativeUrl,
		Bucket:   NoBucket,
		Region:   NoRegion,
		Takeover: false,
	}

	// stop hanging on requests that time out
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	// GET request to url and parse out data
	log.Printf("Sending GET to %s\n", fullUrl)
	resp, err := client.Get(fullUrl)
	if err != nil {
		r.UrlsFailed += 1
		return err
	}
	defer resp.Body.Close()
	bytedata, err := io.ReadAll(resp.Body)
	if err != nil {
		r.UrlsFailed += 1
		return err
	}

	// can successfully ping the endpoint
	r.UrlsProcessed += 1

	/////////////////////////////////
	// FIRST CHECK: Request Headers
	/////////////////////////////////

	log.Println("Starting First Check: Request Headers")

	// skip if Google Cloud headers are present
	if resp.Header.Get("X-GUploader-UploadID") != "" {
		r.UrlsFailed += 1
		return errors.New("Cannot deal with Google Cloud Storage yet.")
	}

	// check for `Server` header to be AmazonS3, but may be changed by proxy or CDN
	server := resp.Header.Get("Server")
	if server == "AmazonS3" {
		status.Bucket = SomeBucket
		log.Println("Detected AWS S3 bucket from URL")
	}

	// check if region is set in headers as well
	region := resp.Header.Get("x-amz-bucket-region")
	if region != "" {
		status.Region = region
		log.Println("Detected AWS S3 bucket region from URL")
	}

	///////////////////////////////
	// SECOND CHECK: CNAME Records
	///////////////////////////////

	log.Println("Starting Second Check: CNAME Records")

	// check if URL points to a S3 URL in any CNAME records. A bucket may use a CDN that
	// masks the original S3 URL, so this may not return anything even if it is a bucket
	potentialCname, _ := GetCNAME(relativeUrl)
	if strings.Contains(potentialCname, ".amazonaws.com") {

		log.Println("Found AWS URL in CNAME, parsing further")

		// s3-<REGION>.amazonaws.com/<BUCKET_NAME>/<OBJECTS>
		expr1 := regexp.MustCompile(`s3-(?P<region>[^.]+).amazonaws.com/(?P<bucket>[^/]+)`)
		expr1Matches := expr1.FindStringSubmatch(potentialCname)
		if len(expr1Matches) != 0 {
			status.Region = expr1Matches[1]
			status.Bucket = expr1Matches[2]
			log.Printf("Matched: s3-%s.amazonaws.com/%s\n", status.Region, status.Bucket)
		}

		// <BUCKET_NAME>.s3.<REGION>.amazonaws.com/<OBJECTS>
		expr2 := regexp.MustCompile(`(?P<bucket>[^/]+).s3.(?P<region>[^.]+).amazonaws.com`)
		expr2Matches := expr2.FindStringSubmatch(potentialCname)
		if len(expr2Matches) != 0 {
			status.Region = expr2Matches[2]
			status.Bucket = expr2Matches[1]
			log.Printf("Matched: %s.s3.%s.amazonaws.com\n", status.Bucket, status.Region)
		}

		// shouldn't happen, but continue checks if bucket name couldn't be found
		if status.Bucket == NoBucket {
			log.Println("Continuing checks, parsing CNAME didn't work out")
			goto bodyCheck
		}

		// if bucket name found but no region, region must be us-east-1
		if status.Bucket != NoBucket && status.Region == NoRegion {
			status.Region = "us-east-1"
		}

		// otherwise do a quick takeover check and return.
		log.Println("Checking for takeover")
		if strings.Contains(string(bytedata), "NoSuchBucket") {
			r.TakeoverPossible += 1
			status.Takeover = true
			log.Println("Takeover is possible for parsed bucket")
		}

		log.Println("Adding successful entry and returning")
		r.Endpoints += 1
		r.Buckets = append(r.Buckets, status)
		return nil
	}

bodyCheck:

	///////////////////////////////////
	/// THIRD CHECK: URL AS BUCKET NAME
	///////////////////////////////////

	log.Println("Starting Third Check: URL as Bucket Name")

	// status.Region being set helps make this faster, otherwise will enumerate through all regions
	if val, region := CheckBucketExists(relativeUrl, status.Region); val {
		status.Bucket = relativeUrl
		status.Region = region
	}

	///////////////////////////////////
	/// FINAL CHECK: HTTP XML RESPONSE
	///////////////////////////////////

	// attempt to serialize into proper XML, if not, return
	xml := etree.NewDocument()
	if err := xml.ReadFromBytes(bytedata); err != nil {
		goto end
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
			r.TakeoverPossible += 1

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
		log.Println("Starting Final Check: Parsing Open Bucket")
		status.Bucket = resTag.SelectElement("Name").Text()
	}

end:

	// if name isn't unknown increment endpoint
	if status.Bucket != NoBucket {
		r.Endpoints += 1
	}

	r.Buckets = append(r.Buckets, status)
	return nil
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

func (r *Resolver) Table() [][]string {
	var contents [][]string
	for _, status := range r.Buckets {
		if status.Bucket != NoBucket {
			contents = append(contents, status.Row())
		}
	}
	return contents
}

// Finalize by writing bucket names to a filepath, and displaying stats to user.
func (r *Resolver) OutputStats(path string) error {
	// if path is specified write bucket names to path
	if path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()

		// write each entry as a line, ignore takeovers since they don't exist
		writer := bufio.NewWriter(file)
		for _, data := range r.Buckets {
			if !data.Takeover && data.Bucket != SomeBucket {
				_, _ = writer.WriteString(data.Bucket + "\n")
			}
		}
		writer.Flush()
	}

	var nameCount int
	for _, data := range r.Buckets {
		if data.Bucket != SomeBucket {
			nameCount += 1
		}
	}

	// output rest of the stats
	fmt.Printf("\nURLs Processed: %d\n", r.UrlsProcessed)
	fmt.Printf("URLs Failed: %d\n\n", r.UrlsFailed)
	fmt.Printf("S3 Endpoints Found: %d\n", r.Endpoints)
	fmt.Printf("Bucket Names Identified: %d\n", nameCount)
	fmt.Printf("Bucket Takeovers Possible: %d\n\n", r.TakeoverPossible)
	return nil
}
