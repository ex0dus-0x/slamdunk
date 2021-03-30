package slamdunk

import (
    "log"
    "errors"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

// Maps a bucket name to another map of actions and whether they are set
type Audit map[string]map[string]bool

// Represents a single auditor session, where a playbook is constructed from a configuration
// and applied against single buckets, and bulk results can be outputted.
type Auditor struct {
    Playbook    map[string]Action
    Results     Audit
}

// Instantiate a new auditor based on the action specified
func NewAuditor(actions string) *Auditor {
    // create a new empty playbook
    var playbook PlayBook
    if actions == "all" {
        playbook = NewPlayBook()
    }

    // empty map stores the results for all buckets analyzed in this session
    results := Audit{}

    return &Auditor {
        Playbook: playbook,
        Results: results,
    }
}

// Run configured auditor on a single bucket name, and store results in map for output.
func (a *Auditor) Run(bucket string) error {

    // check first if bucket actually exists
    val, region := CheckBucketExists(bucket, NoRegion)
    if !val {
        return errors.New("Specified bucket does not exist in any region.")
    }

    log.Printf("%s found in %s region\n", bucket, region)

    // indicate whether the user is authenticated or not

    // initialize new session for use against all playbook actions
    sess, _ := session.NewSession(&aws.Config{
        Region: aws.String(region)},
    )
    svc := s3.New(sess)
    if svc == nil {
        return errors.New("Could not instantiate new S3 client")
    }

    // run all actions specified in our playbook
    audit := map[string]bool{}
    for name, action := range a.Playbook {
        log.Printf("Testing %s\n", name)
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
