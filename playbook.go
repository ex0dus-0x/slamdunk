package slamdunk

const (
    TempObject = "temporary.txt"
)

// Encapsulates all of the actions we can execute against a target bucket.
type PlayBook map[string]Action

// Implementation of a specific heuristic we want to check for against a target.
type Action struct {
    Description    string
    Cmd            string
    Callback       func(string) bool
}

func (a *Action) TableEntry(name string) []string {
    return []string{name, a.Description, "aws s3api " + a.Cmd}
}

func NewPlayBook() PlayBook {
    return map[string]Action {
        "ListObjects": Action {
            Description: "Read and enumerate over objects in bucket.",
            Cmd: "list-objects --bucket <NAME>",
            Callback: func(name string) bool {
                return false
            },
        },

        "PutObjects": Action {
            Description: "Write object to bucket with key.",
            Cmd: "put-object --bucket <NAME> --key <KEY> --body <FILE>",
            Callback: func(name string) bool {
                return false
            },
        },
    }
}
