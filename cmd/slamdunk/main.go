package main

import (
    "os"
    "log"
    "bufio"
    "errors"
    "strconv"
    "syscall"
    "os/signal"

    "github.com/urfave/cli/v2"
    "github.com/olekukonko/tablewriter"
    "github.com/ex0dus-0x/slamdunk"
)

// Helper used to read out URLs or buckets from a filepath and return as a slice of strings.
func ReadLines(path string) (*[]string, error) {
    // read file from path
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // read path into lines
    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return &lines, scanner.Err()
}

// Helper to render and output an ASCII table
func OutputTable(outputMap [][]string, header []string) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader(header)
    for _, v := range outputMap {
        table.Append(v)
    }
    table.Render()
}

func main() {
    app := &cli.App {
        Name: "slamdunk",
        Usage: "AWS S3 Bucket Permissions Auditor",
        Commands: []*cli.Command {
            {
                Name: "audit",
                Usage: "Given bucket name(s), and/or a file with newline-seperated bucket names, audit permissions.",
                Flags: []cli.Flag {
                    &cli.StringSliceFlag {
                        Name: "name",
                        Usage: "Name of a target S3 bucket to audit. Can be invoked multiple times.",
                        Aliases: []string{"n"},
                    },
                    &cli.StringFlag {
                        Name: "file",
                        Usage: "File with multiple target bucket names to audit.",
                        Aliases: []string{"f"},
                    },
                    &cli.StringFlag {
                        Name: "action",
                        Usage: "Run only a specific action from the playbook against the bucket(s).",
                        Aliases: []string{"a"},
                        Value: "all",
                    },
                },
                Action: func(c *cli.Context) error {

                    // required arguments
                    names := c.StringSlice("name")
                    file := c.String("file")
                    if len(names) == 0 && file == "" {
                        return errors.New("Must specify both or either `--name` or `--file`.")
                    }

                    // if file specified, append to bucket names
                    if file != "" {
                        vals, err := ReadLines(file)
                        if err != nil {
                            return err
                        }
                        names = append(names, *vals...)
                    }

                    //action := c.String("action")
                    return nil
                },
            },
            {
                Name: "resolve",
                Usage: "Given a URL, and/or a file of URLs, attempt to resolve a bucket name, while testing for takeover capabilities",
                Flags: []cli.Flag {
                    &cli.StringSliceFlag {
                        Name: "url",
                        Usage: "URL to resolve a bucket name from. Can be invoked multiple times.",
                        Aliases: []string{"n"},
                    },
                    &cli.StringFlag {
                        Name: "file",
                        Usage: "File with multiple normal URLs names to resolve.",
                        Aliases: []string{"f"},
                    },
                    &cli.BoolFlag {
                        Name: "matches",
                        Usage: "Display only URLs that resolve to a valid bucket (default is set).",
                        Aliases: []string{"m"},
                        Value: true,
                    },
                },
                Action: func(c *cli.Context) error {

                    // required arguments
                    urls := c.StringSlice("url")
                    file := c.String("file")
                    if len(urls) == 0 && file == "" {
                        return errors.New("Must specify both or either `--url` or `--file`.")
                    }

                    matches := c.Bool("matches")

                    // if file specified, append to URLs
                    if file != "" {
                        vals, err := ReadLines(file)
                        if err != nil {
                            return err
                        }
                        urls = append(urls, *vals...)
                    }

                    // stores contents for making an ASCII table
                    outputMap := [][]string{}
                    header := []string{"URL", "S3 Bucket", "Bucket Takeover?"}

                    // handle keyboard interrupts to output table with content so far
                    channel := make(chan os.Signal)
                    signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
                    go func() {
                        <-channel
                        log.Println("Ctrl+C pressed, interrupting execution...")
                        OutputTable(outputMap, header)
                        os.Exit(0)
                    }()

                    // resolve each and parse output for display
                    for _, url := range urls {
                        log.Printf("Attempting to resolve %s...\n", url)
                        resolved, err := slamdunk.Resolver(url)
                        if err != nil {
                            log.Print(err)
                            continue
                        }

                        // identify string to output based on result parsed
                        var output string
                        if resolved.Bucket == nil {

                            // is matches is switched, skip over
                            if matches {
                                continue
                            }
                            output = "Not Found"

                        // empty string means S3 is present, but name cannot be found
                        } else if *resolved.Bucket == "" {
                            output = "Some S3 Bucket"

                        // otherwise bucket is found
                        } else {
                            output = *resolved.Bucket
                        }

                        row := []string{url, output, strconv.FormatBool(resolved.Takeover)}
                        outputMap = append(outputMap, row)
                    }
                    OutputTable(outputMap, header)
                    return nil
                },
            },
            {
                Name: "playbook",
                Usage: "List all supported actions in the playbook, and provide additional information about their use",
                Flags: []cli.Flag {
                    &cli.StringFlag {
                        Name: "action",
                        Usage: "If set, prints information only about specific action in playbook.",
                        Aliases: []string{"f"},
                    },
                },
                Action: func(c *cli.Context) error {
                    return nil
                },
            },
        },
    }

    err := app.Run(os.Args)
    if err != nil {
        log.Fatal(err)
    }
}
