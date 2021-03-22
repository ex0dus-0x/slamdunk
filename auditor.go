package slamdunk

import (
    "fmt"
    "errors"
)

// Runs a specific action, or all actions from the playbook against the
// bucket(s) that are parsed out within the auditor object.
func Auditor(bucket string, action string) error {

    // check first if bucket actually exists
    ok, region := CheckBucketExists(bucket, NoRegion)
    if ok != true {
        return errors.New("Specified bucket does not exist in any region.")
    }

    fmt.Println(region)

    // indicate whether the user is authenticated or not


    return nil
}
