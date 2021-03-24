package slamdunk

import (
    "errors"
)

type Auditor struct {
    playbook map[string]Action
}

// Run configured auditor on bucket and store output
func (a *Auditor) Run(bucket string) error {

    // check first if bucket actually exists
    if !CheckBucketExists(bucket, NoRegion) {
        return errors.New("Specified bucket does not exist in any region.")
    }

    // indicate whether the user is authenticated or not

    return nil
}
