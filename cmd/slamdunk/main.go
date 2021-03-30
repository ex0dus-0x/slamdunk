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
func PrintTable(header []string, content [][]string) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader(header)
    table.SetAutoMergeCellsByColumnIndex([]int{0})
    table.SetRowLine(true)
    table.SetAutoWrapText(false)
    table.AppendBulk(content)
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

                    header := []string{"Bucket Name", "Permission", "Enabled?"}

                    // audit each bucket and handle accordingly
                    auditor := slamdunk.NewAuditor("all")

                    // handle keyboard interrupts to output table with content so far
                    channel := make(chan os.Signal)
                    signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
                    go func() {
                        <-channel
                        log.Println("Ctrl+C pressed, interrupting execution...")
                        PrintTable(header, auditor.Table())
                        os.Exit(0)
                    }()

                    for _, bucket := range names {
                        log.Printf("Auditing %s...\n", bucket)
                        if err := auditor.Run(bucket); err != nil {
                            return err
                        }
                    }

                    PrintTable(header, auditor.Table())
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

                    outputPath := c.String("output")

                    // stores contents for making an ASCII table
                    header := []string{"URL", "Bucket Name", "Region", "Vulnerable to Takeover?"}

                    // actual object that interfaces with resolving
                    resolver := slamdunk.NewResolver()

                    // handle keyboard interrupts to output table with content so far
                    channel := make(chan os.Signal)
                    signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
                    go func() {
                        <-channel
                        log.Println("Ctrl+C pressed, interrupting execution...")
                        PrintTable(header, resolver.Table())
                        if err := resolver.OutputStats(outputPath); err != nil {
                            log.Fatal(err)
                        }
                        os.Exit(0)
                    }()

                    // resolve each and parse output for display
                    for _, url := range urls {
                        log.Printf("Attempting to resolve %s...\n", url)
                        err := resolver.Resolve(url)
                        if err != nil {
                            log.Println(err)
                            continue
                        }
                    }
                    PrintTable(header, resolver.Table())
                    if err := resolver.OutputStats(outputPath); err != nil {
                        return err
                    }
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
                    table := [][]string{}

                    // search for action if specified
                    actionName := c.String("action")
                    if actionName != "" {
                        if action, ok := playbook[actionName]; ok {
                            table = append(table, action.TableEntry(actionName))
                        } else {
                            return errors.New("Cannot find specified action in playbook.")
                        }
                    } else {
                        for name, action := range playbook {
                            table = append(table, action.TableEntry(name))
                        }
                    }

                    header := []string{"Action", "Description", "Equivalent Command"}
                    PrintTable(header, table)
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
