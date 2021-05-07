package slamdunk

import (
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/fatih/color"
)

// Maps a bucket name to another map of actions and whether they are set
type Audit map[string]map[string]bool

// Represents a single auditor session, where a playbook is constructed from a configuration
// and applied against single buckets, and bulk results can be outputted.
type Auditor struct {
	// name of the IAM profile we're operating on
	Profile string

	// stores all the actions we care about testing against the buckets
	Playbook map[string]Action

	// map stores the results for all buckets analyzed in this session
	Results Audit
}

// Instantiate a new auditor based on the actions specified. Empty slice means run all.
func NewAuditor(actions []string, profile string) (*Auditor, error) {

	// check IAM metadata
	fmt.Printf("\nYou are: ")
	c := color.New(color.FgCyan).Add(color.Underline)
	if !IsAuthenticated() {
		c.Println("UNAUTHENTICATED")
	} else {
		// get ARN from profile, if not possible then error
		log.Println("Parsing out current IAM profile's ARN")
		arn, err := GetIAMUserARN(profile)
		if err != nil {
			return nil, err
		}
		c.Println(arn)
	}
	fmt.Println()

	// if specific actions, clear playbook of those we don't care about
	log.Println("Creating playbook based on actions to run")
	playbook := NewPlayBook()
	if len(actions) != 0 {
		temp := PlayBook{}
		for _, action := range actions {
			if val, ok := playbook[action]; ok {
				temp[action] = val
			}
		}
		playbook = temp
	}

	results := Audit{}
	return &Auditor{
		Profile:  profile,
		Playbook: playbook,
		Results:  results,
	}, nil
}

// Run configured auditor on a single bucket name, and store results in map for output.
func (a *Auditor) Run(bucket string) error {

	// check first if bucket actually exists
	log.Println("Checking if bucket exists and finding region")
	val, region := CheckBucketExists(bucket, NoRegion)
	if !val {
		return errors.New("Specified bucket does not exist in any region.")
	}
	log.Printf("%s found in %s region\n", bucket, region)

	// initialize session for use with parsed region against all playbook actions
	log.Println("Creating main session for auditing permissions")
	sess, _ := session.NewSessionWithOptions(session.Options{
		Profile: a.Profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
	})
	svc := s3.New(sess)
	if svc == nil {
		return errors.New("Could not instantiate new S3 client")
	}

	// run all actions specified in our playbook
	audit := map[string]bool{}
	for name, action := range a.Playbook {
		log.Printf("Testing %s against %s\n", name, bucket)
		audit[name] = action.Callback(*svc, bucket)
	}
	a.Results[bucket] = audit
	return nil
}

// Creates a tabulated version of the auditor results after processing buckets
func (a *Auditor) Table() [][]string {
	content := [][]string{}
	for bucket, action := range a.Results {
		for perm, result := range action {
			var resultOut string
			if result {
				resultOut = "✔️"
			} else {
				resultOut = "❌"
			}
			content = append(content, []string{bucket, perm, resultOut})
		}
	}
	return content
}
