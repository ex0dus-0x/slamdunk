package main

import (
    "os"
    "log"
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

                    // required arguments
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
                    outputMap := [][]string{}
                    header := []string{"URL", "Bucket Name", "Region", "Bucket Takeover?"}

                    // stores result to write-append to output file

                    // handle keyboard interrupts to output table with content so far
                    channel := make(chan os.Signal)
                    signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
                    go func() {
                        <-channel
                        log.Println("Ctrl+C pressed, interrupting execution...")

                        // on exception, first display what's already stored in output
                        OutputTable(outputMap, header)

                        // then if outputList is populated, write to path specified on disk

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

                        // skip URLs that don't resolve to buckets
                        if c.Bool("matches") && !resolved.HasBucket() {
                            continue
                        }
                        outputMap = append(outputMap, resolved.GenTableRow())
                    }
                    OutputTable(outputMap, header)
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
                    outputMap := [][]string{}
                    header := []string{"Action", "Description", "Equivalent Command"}

                    // search for action if specified
                    actionName := c.String("action")
                    if actionName != "" {
                        if action, ok := playbook[actionName]; ok {
                            outputMap = append(outputMap, action.TableEntry(actionName))
                        } else {
                            return errors.New("Cannot find specified action in playbook.")
                        }
                    } else {
                        for name, action := range playbook {
                            outputMap = append(outputMap, action.TableEntry(name))
                        }
                    }

                    OutputTable(outputMap, header)
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
