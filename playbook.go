package slamdunk

// Encapsulates all of the actions we can execute against a target bucket.
type PlayBook struct {
    Actions map[string]Action
}

// Implementation of a specific heuristic we want to check for against a target.
type Action struct {
    Description    string
    Cmd            string
    Callback       func(string) bool
}

func NewPlayBook() *PlayBook {
    return &PlayBook {
        Actions: map[string]Action {
            "ListObjects": Action {
                Description: "Enumerate over objects without needing to be the owner",
                Cmd: "ls s3://{BUCKET_NAME}",
                Callback: func(name string) bool {
                    return false
                },
            },
        },
    }
}
