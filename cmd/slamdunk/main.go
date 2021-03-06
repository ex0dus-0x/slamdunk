package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ex0dus-0x/slamdunk"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
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
	app := &cli.App{
		Name:  "slamdunk",
		Usage: "Cloud Storage Permissions Auditor",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "If set, will print out log for debugging.",
				Aliases: []string{"v"},
			},
		},
		Commands: []*cli.Command{
			{
				Name: "audit",
				Usage: `Given bucket name(s), and/or a file with newline-seperated bucket names, audit permissions. 
                By default, only READ-based permissions will be tested against each bucket specified.`,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "name",
						Usage:   "Name of a target S3 bucket to audit. Can be invoked multiple times.",
						Aliases: []string{"n"},
					},
					&cli.StringFlag{
						Name:    "file",
						Usage:   "File with multiple target bucket names to audit.",
						Aliases: []string{"f"},
					},
					&cli.BoolFlag{
						Name:    "list",
						Usage:   "Get buckets that can be listed for the given scoped IAM principal, if ListBucket is allowed.",
						Aliases: []string{"l"},
					},
					&cli.StringSliceFlag{
						Name:    "perm",
						Usage:   "Runs only specified permission against buckets. Can be invoked multiple times.",
						Aliases: []string{"p"},
					},
					&cli.BoolFlag{
						Name:    "write",
						Usage:   "Run checks on WRITE permissions (WARNING: may alter content/configurations of configuration resources).",
						Aliases: []string{"w"},
					},
					&cli.StringFlag{
						Name:        "profile",
						Usage:       "Specifies an IAM profile to be used when auditing buckets. Use 'none' to test without any profiles.",
						Value:       "default",
						DefaultText: "default",
						Aliases:     []string{"i"},
					},
				},
				Action: func(c *cli.Context) error {
					if !c.Bool("verbose") {
						log.SetOutput(ioutil.Discard)
					}
					log.Printf("Starting slamdunk.")

					// IAM profile check
					profile := c.String("profile")
					if profile == "none" {
						profile = ""
					}
					log.Println("Using IAM profile", profile)

					// argparse out buckets to test
					log.Println("Argparsing for bucket names to audit")
					names := c.StringSlice("name")
					file := c.String("file")
					list := c.Bool("list")
					if len(names) == 0 && file == "" && !list {
						return errors.New("Must specify all, some or one of `--name`, `--file`, or `--list`.")
					}

					// if file specified, append to bucket names
					if file != "" {
						vals, err := ReadLines(file)
						if err != nil {
							return err
						}
						names = append(names, *vals...)
					}

					// if `--list` is set, grab buckets for current IAM principal, otherwise exit if denied
					if list {
						log.Println("Checking if we can parse buckets with ListBucket")
						listed, err := slamdunk.ListBuckets(profile)
						if err != nil {
							return err
						}
						names = append(names, *listed...)
					}

					log.Printf("Parsed out %d buckets for testing\n", len(names))

					// parse specific actions
					actions := []string{}
					if len(c.StringSlice("perm")) != 0 {
						actions = c.StringSlice("perm")
					}
                    log.Println("Running actions", actions);

					// audit each bucket and handle accordingly
					auditor, err := slamdunk.NewAuditor(actions, profile)
					if err != nil {
						return err
					}

					// handle keyboard interrupts to output table with content so far
					log.Println("Installing signal handler to handle interrupts")
					channel := make(chan os.Signal)
					signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
					go func() {
						<-channel
						log.Println("Ctrl+C pressed, interrupting execution...")
						os.Exit(0)
					}()

					for _, bucket := range names {
						log.Printf("Auditing %s...\n", bucket)
						if err := auditor.Run(bucket); err != nil {
							return err
						}
					}

					auditor.Output()
					return nil
				},
			},
			{
				Name:  "resolve",
				Usage: "Given a URL, and/or a file of URLs, attempt to resolve a bucket name, while testing for takeover capabilities",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "url",
						Usage:   "URL to resolve a bucket name from. Can be invoked multiple times.",
						Aliases: []string{"n"},
					},
					&cli.StringFlag{
						Name:    "file",
						Usage:   "File with multiple normal URLs names to resolve.",
						Aliases: []string{"f"},
					},
					&cli.BoolFlag{
						Name:    "matches",
						Usage:   "Display only URLs that resolve to a bucket (default is true).",
						Aliases: []string{"m"},
						Value:   true,
					},
					&cli.StringFlag{
						Name:    "output",
						Usage:   "Path where resultant buckets only are stored, seperated by newline.",
						Aliases: []string{"o"},
					},
				},
				Action: func(c *cli.Context) error {
					if !c.Bool("verbose") {
						log.SetOutput(ioutil.Discard)
					}
					log.Printf("Starting slamdunk.")

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
					log.Printf("Number of URLs parsed for processing: %d\n", len(urls))

					outputPath := c.String("output")

					// stores contents for making an ASCII table
					header := []string{"URL", "Bucket Name", "Region", "Vulnerable to Takeover?"}

					// actual object that interfaces with resolving
					resolver := slamdunk.NewResolver()

					// handle keyboard interrupts to output table with content so far
					log.Println("Installing signal handler to handle interrupts")
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
				Name:  "playbook",
				Usage: "List supported actions in the playbook, and provide additional information about their use",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "action",
						Usage:   "If set, prints information only about specific action in playbook.",
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
		fmt.Println("ERROR:", err)
	}
}
