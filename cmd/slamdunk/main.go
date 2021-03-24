package main

import (
    "os"
    "log"
    "fmt"
    "bufio"
    "errors"
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

type ResolveStats struct {
    // buckets successfully parsed out
    Buckets             []string

    // number of URLs successfully processed
    UrlsProcessed       int

    // number of URLS failed to process (ie timeout)
    UrlsFailed          int

    // S3 endpoints identified, even if name can't be found
    Endpoints           int

    // how many endpoints can be taken over
    TakeoverPossible    int
}

// Finalize by writing bucket names to a filepath, and displaying stats to user.
func (r *ResolveStats) Output(path string) error {

    // if path is specified write bucket names to path
    if path != "" {
        file, err := os.OpenFile(path, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
        if err != nil {
            return err
        }
        defer file.Close()

        // write each entry as a line
        writer := bufio.NewWriter(file)
        for _, data := range r.Buckets {
            _, _ = writer.WriteString(data + "\n")
        }
        writer.Flush()
    }

    // output rest of the stats
    fmt.Printf("\nURLs Processed: %d\n", r.UrlsProcessed)
    fmt.Printf("URLs Failed: %d\n\n", r.UrlsFailed)
    fmt.Printf("S3 Endpoints Found: %d\n", r.Endpoints)
    fmt.Printf("Bucket Names Identified: %d\n", len(r.Buckets))
    fmt.Printf("Bucket Takeovers Possible: %d\n\n", r.TakeoverPossible)
    return nil
}

// Helper to render and output an ASCII table
func OutputTable(outputTable [][]string, header []string) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader(header)
    for _, v := range outputTable {
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
                Usage: `Given bucket name(s), and/or a file with newline-seperated bucket names, audit permissions. 
                By default, READ-based permissions will be run only to ensure no detrimental changes are made to the bucket.`,
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
                        Name: "enable",
                        Usage: "Enable another action to run alongside the default checks.",
                    },
                    &cli.StringFlag {
                        Name: "enable-set",
                        Usage: "Enable another set of actions to run alongside the default checks.",
                    },
                    &cli.StringFlag {
                        Name: "only",
                        Usage: "Run only a specific action from the playbook against the bucket(s).",
                    },
                },
                Action: func(c *cli.Context) error {
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

                    // stores contents for making an ASCII table
                    outputTable := [][]string{}
                    header := []string{"Bucket Name", "Permission", "Enabled?"}

                    // audit each bucket and handle accordingly
                    // TODO
                    for _, bucket := range names {
                        log.Printf("Auditing %s...\n", bucket)
                    }

                    OutputTable(outputTable, header)
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
                        Usage: "Display only URLs that resolve to a bucket (default is true).",
                        Aliases: []string{"m"},
                        Value: true,
                    },
                    &cli.StringFlag {
                        Name: "output",
                        Usage: "Path where resultant buckets only are stored, seperated by newline.",
                        Aliases: []string{"o"},
                    },
                },
                Action: func(c *cli.Context) error {
                    urls := c.StringSlice("url")
                    file := c.String("file")
                    if len(urls) == 0 && file == "" {
                        return errors.New("Must specify both or either `--url` or `--file`.")
                    }

                    // if file specified, append to URLs
                    if file != "" {
                        vals, err := ReadLines(file)
                        if err != nil {
                            return err
                        }
                        urls = append(urls, *vals...)
                    }

                    // stores contents for making an ASCII table
                    outputTable := [][]string{}
                    header := []string{"URL", "Bucket Name", "Region", "Vulnerable to Takeover?"}

                    // processed stats from resolving
                    var stats ResolveStats

                    // handle keyboard interrupts to output table with content so far
                    channel := make(chan os.Signal)
                    signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
                    go func() {
                        <-channel
                        log.Println("Ctrl+C pressed, interrupting execution...")

                        // on exception, first display what's already stored in output
                        OutputTable(outputTable, header)
                        stats.Output(c.String("output"))
                        os.Exit(0)
                    }()

                    // resolve each and parse output for display
                    for _, url := range urls {
                        log.Printf("Attempting to resolve %s...\n", url)
                        resolved, err := slamdunk.Resolver(url)
                        if err != nil {
                            log.Print(err)
                            stats.UrlsFailed += 1
                            continue
                        }

                        // successfully processed a URL
                        stats.UrlsProcessed += 1

                        // skip URLs that don't resolve to buckets
                        if c.Bool("matches") && !resolved.HasBucket() {
                            continue
                        }

                        // successfully processed a S3 endpoint
                        stats.Endpoints += 1
                        outputTable = append(outputTable, resolved.GenTableRow())

                        // skip adding to output list if it doesn't have name
                        if resolved.Bucket != slamdunk.SomeBucket {
                            stats.Buckets = append(stats.Buckets, resolved.Bucket)
                        }
                        if resolved.Takeover {
                            stats.TakeoverPossible += 1
                        }
                    }

                    // output table and stats, write file if option specified
                    OutputTable(outputTable, header)
                    stats.Output(c.String("output"))
                    return nil
                },
            },
            {
                Name: "playbook",
                Usage: "List supported actions in the playbook, and provide additional information about their use",
                Flags: []cli.Flag {
                    &cli.StringFlag {
                        Name: "action",
                        Usage: "If set, prints information only about specific action in playbook.",
                        Aliases: []string{"a"},
                    },
                },
                Action: func(c *cli.Context) error {
                    playbook := slamdunk.NewPlayBook()

                    // stores contents for making an ASCII table
                    outputTable := [][]string{}
                    header := []string{"Action", "Description", "Equivalent Command"}

                    // search for action if specified
                    actionName := c.String("action")
                    if actionName != "" {
                        if action, ok := playbook[actionName]; ok {
                            outputTable = append(outputTable, action.TableEntry(actionName))
                        } else {
                            return errors.New("Cannot find specified action in playbook.")
                        }
                    } else {
                        for name, action := range playbook {
                            outputTable = append(outputTable, action.TableEntry(name))
                        }
                    }

                    OutputTable(outputTable, header)
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
