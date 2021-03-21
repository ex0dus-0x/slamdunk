package slamdunk

import (
    "errors"
)

// Runs a specific action, or all actions from the playbook against the
// bucket(s) that are parsed out within the auditor object.
func Auditor(bucket string, action string) error {

    // check first if bucket actually exists
    if CheckBucketExists(bucket) != true {
        return errors.New("Specified bucket does not exist")
    }

    return nil
}
