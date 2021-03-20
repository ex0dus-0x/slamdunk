package slamdunk

import (
    "os"
    "fmt"
    "os/user"
)


// Helper used to check if the current user is authenticated, as some permissions are configured
// to work only if its by an authenticated user. We won't parse the credentials if it exists, as the
// S3 SDK should be doing that for us.
func IsAuthenticated() bool {
    // resolve standard path to where credentials should be
    user, _ := user.Current()
    dir := user.HomeDir
    path := fmt.Sprintf("%s/.aws/credentials", dir)

    // filepath check
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return false
    }
    return true
}


// Runs a specific action, or all actions from the playbook against the
// bucket(s) that are parsed out within the auditor object.
func Auditor(bucket string, action string) error {
    return nil
}
