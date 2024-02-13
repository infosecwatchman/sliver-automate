package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/bishopfox/sliver/client/console"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"github.com/rodaine/table"
	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func menuCommands() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "",
		Short: "",
		Long:  ``,
	}
	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search for beacons with given regex.",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("search called")
		},
	}
	triggerCmd := &cobra.Command{
		Use:   "trigger",
		Short: "A brief description of your command",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("trigger called")
		},
	}
	interactCmd := &cobra.Command{
		Use:   "interact",
		Short: "Use interact to perform actions on multiple beacons/sessions simultaneously.",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			app.Printf("Must select either `beacon` or `session`.")
			cmd.Help()
		},
	}
	selectBeaconCmd := &cobra.Command{
		Use:   "beacon",
		Short: "interact with beacons",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			filter := cmd.Flag("filter").Value.String()

			var filterRegex *regexp.Regexp
			if cmd.Flag("filter-re").Value.String() != "" {
				var err error
				filterRegex, err = regexp.Compile(cmd.Flag("filter-re").Value.String())
				if err != nil {
					app.Printf("%s\n", err)
					return
				}
			}
			beacons, err := client.rpc.GetBeacons(context.Background(), &commonpb.Empty{})
			if err != nil {
				app.Printf("Error in getting beacons: %s", err)
			}
			if len(beacons.Beacons) == 0 {
				app.Printf("No beacons 🙁\n")
				return
			} else {
				var filteredBeacons []string
				tbl := table.New("ID", "Name", "Transport", "RemoteAddress", "Hostname", "Username", "OS")
				for _, beacon := range beacons.Beacons {
					filteredBeacon := []string{
						strings.Split(beacon.ID, "-")[0],
						beacon.Name,
						beacon.Transport,
						beacon.RemoteAddress,
						beacon.Hostname,
						strings.TrimPrefix(beacon.Username, beacon.Hostname+"\\"),
						fmt.Sprintf("%s/%s", beacon.OS, beacon.Arch),
					}
					//filteredBeacon = [strings.Split(beacon.ID, "-")[0], beacon.Name, ]
					if filter == "" && filterRegex == nil {
						tbl.AddRow(strings.Split(beacon.ID, "-")[0],
							beacon.Name,
							beacon.Transport,
							beacon.RemoteAddress,
							beacon.Hostname,
							strings.TrimPrefix(beacon.Username, beacon.Hostname+"\\"),
							fmt.Sprintf("%s/%s", beacon.OS, beacon.Arch))
						filteredBeacons = append(filteredBeacons, beacon.ID)
					} else {
						for _, rowEntry := range filteredBeacon {
							if filter != "" {
								if strings.Contains(rowEntry, filter) {
									tbl.AddRow(strings.Split(beacon.ID, "-")[0],
										beacon.Name,
										beacon.Transport,
										beacon.RemoteAddress,
										beacon.Hostname,
										strings.TrimPrefix(beacon.Username, beacon.Hostname+"\\"),
										fmt.Sprintf("%s/%s", beacon.OS, beacon.Arch))
									filteredBeacons = append(filteredBeacons, beacon.ID)
									break
								}
							}
							if filterRegex != nil {
								if filterRegex.MatchString(rowEntry) {
									tbl.AddRow(strings.Split(beacon.ID, "-")[0],
										beacon.Name,
										beacon.Transport,
										beacon.RemoteAddress,
										beacon.Hostname,
										strings.TrimPrefix(beacon.Username, beacon.Hostname+"\\"),
										fmt.Sprintf("%s/%s", beacon.OS, beacon.Arch))
									filteredBeacons = append(filteredBeacons, beacon.ID)
									break
								}
							}
						}
					}
					//tbl.AddRow(beacon.ID, beacon.Name, beacon.RemoteAddress, beacon.PID, beacon.Filename, beacon.Username, beacon.OS, beacon.IsDead, next)
				}
				app.Printf("%d beacons selected...\n", len(filteredBeacons))
				if len(filteredBeacons) <= 5 {
					tbl.Print()
					time.Sleep(10 * time.Millisecond)
				}

				ctx = context.WithValue(context.Background(), "beaconTable", tbl)
				ctx = context.WithValue(ctx, "beacons", filteredBeacons)
				//fmt.Printf("#####: %s", ctx.Value("beacons"))
				app.SwitchMenu("interact")
			}

		},
	}
	selectBeaconCmd.Flags().StringP("filter", "f", "", "filter beacons by substring")
	selectBeaconCmd.Flags().StringP("filter-re", "e", "", "filter beacons by regular expression")
	interactCmd.AddCommand(selectBeaconCmd)

	exitCmd := &cobra.Command{
		Use:   "exit",
		Short: "Exit sliver-automate console.",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(1)
		},
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all active Beacons",
		Long:  `List all active Beacons`,
		Run: func(cmd *cobra.Command, args []string) {
			table.DefaultHeaderFormatter = func(format string, vals ...interface{}) string {
				return strings.ToUpper(fmt.Sprintf(format, vals...))
			}
			/*
				tbl := table.New("ID", "Name", "Cost ($)")

				for _, widget := range Widgets {
					tbl.AddRow(widget.ID, widget.Name, widget.Cost)
				}
			*/
			tbl := table.New("ID", "Name", "RemoteAddress", "PID", "Filename", "Username", "OS", "IsDead", "NextCheckin")

			beacons, err := client.rpc.GetBeacons(context.Background(), &commonpb.Empty{})
			if err != nil {
				app.Printf("Error in getting beacons: %s", err)
			}
			if len(beacons.Beacons) == 0 {
				app.Printf("No beacons 🙁\n")
				return
			} else {
				for _, beacon := range beacons.Beacons {
					nextCheckin := time.Unix(beacon.NextCheckin, 0)
					//nextCheckinDateTime := nextCheckin.Format(time.UnixDate)

					var next string
					var interval string

					if time.Unix(beacon.NextCheckin, 0).Before(time.Now()) {
						interval = time.Since(nextCheckin).Round(time.Second).String()
						next = fmt.Sprintf("%s%s%s", console.Bold+console.Red, interval, console.Normal)
					} else {
						interval = time.Until(nextCheckin).Round(time.Second).String()
						next = fmt.Sprintf("%s%s%s", console.Bold+console.Green, interval, console.Normal)
					}
					tbl.AddRow(beacon.ID, beacon.Name, beacon.RemoteAddress, beacon.PID, beacon.Filename, beacon.Username, beacon.OS, beacon.IsDead, next)
				}
			}
			tbl.Print()
		},
	}
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(triggerCmd)
	rootCmd.AddCommand(interactCmd)
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(listCmd)
	for _, cmd := range rootCmd.Commands() {
		c := carapace.Gen(cmd)

		if cmd.Args != nil {
			c.PositionalAnyCompletion(
				carapace.ActionCallback(func(c carapace.Context) carapace.Action {
					return carapace.ActionFiles()
				}),
			)
		}

		flagMap := make(carapace.ActionMap)
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "file" || strings.Contains(f.Usage, "file") {
				flagMap[f.Name] = carapace.ActionFiles()
			}
		})

		c.FlagCompletion(flagMap)
	}

	rootCmd.InitDefaultHelpCmd()

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.DisableFlagsInUseLine = true
	return rootCmd
}

func interactBeaconCommands() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "",
		Short: "",
		Long:  ``,
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list beacons currently being interacted with.",
		Run: func(cmd *cobra.Command, args []string) {
			app.Printf("%d beacons selected: \n", len(ctx.Value("beacons").([]string)))
			var tbl table.Table = ctx.Value("beaconTable").(table.Table)
			tbl.Print()
		},
	}
	rootCmd.AddCommand(listCmd)
	killCmd := &cobra.Command{
		Use:   "kill",
		Short: "kill beacon(s)",
		Run: func(cmd *cobra.Command, args []string) {
			var beacons interface{}
			beacons = ctx.Value("beacons")
			var parsedBeacons []string = beacons.([]string)
			var longestbeacon int
			for _, beacon := range parsedBeacons {
				_, err := client.rpc.Kill(context.Background(), &sliverpb.KillReq{
					Force: true,
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(60),
						BeaconID: beacon,
					},
				})
				if err != nil {
					log.Fatal(err)
				}
				activebeacon, _ := client.rpc.GetBeacon(context.Background(), &clientpb.Beacon{ID: beacon})
				nextCheckin := time.Unix(activebeacon.NextCheckin, 0)
				if !time.Unix(activebeacon.NextCheckin, 0).Before(time.Now()) {
					if int(time.Until(nextCheckin).Round(time.Second).Seconds())+time.Unix(activebeacon.Jitter, 0).Second() > longestbeacon {
						longestbeacon = int(time.Until(nextCheckin).Round(time.Second).Seconds()) + time.Unix(activebeacon.Jitter, 0).Second()
					}
				}
			}
			app.SwitchMenu("")
			go func(beacons []string) {
				app.Printf("%d beacon(s) will be removed in %ds", len(parsedBeacons), longestbeacon)
				now := time.Now()
				var beaconTaskContext = context.Background()
				var beacongroup sync.WaitGroup
				var timedoutbeacons int = 0
				beacongroup.Add(len(beacons))
				for _, beacon := range beacons {
					go func(beacon string) {
						err := func(beacon string) error {
							tasks, err := client.rpc.GetBeaconTasks(beaconTaskContext, &clientpb.Beacon{ID: beacon})
							for _, task := range tasks.Tasks {
								var check = false
								for {
									if task.State != "sent" {
										time.Sleep(5 * time.Second)
										task, err = client.rpc.GetBeaconTaskContent(beaconTaskContext, &clientpb.BeaconTask{BeaconID: beacon, ID: task.ID})
										if err != nil {
											app.Printf("%s", err)
										}
									}
									if task.State == "sent" {
										break
									}
									if int(time.Since(now).Seconds()) > longestbeacon && check {
										var returnerror = errors.New(fmt.Sprintf("Timeout for beacon %s", beacon))
										return returnerror
									}
									if int(time.Since(now).Seconds()) > longestbeacon {
										check = true
										time.Sleep(5 * time.Second)
									}

								}
							}
							return nil
						}(beacon)
						if err != nil {
							app.Printf("%s", err)
							timedoutbeacons++
						}
						_, err = client.rpc.RmBeacon(context.Background(), &clientpb.Beacon{ID: beacon})
						if err != nil {
							log.Fatal(err)
						}
						beacongroup.Done()
					}(beacon)
				}
				beacongroup.Wait()
				if timedoutbeacons == 0 {
					app.Printf("%d beacon(s) removed.", len(parsedBeacons))
				} else {
					app.Printf("%d beacon(s) removed. %d beacon(s) timedout", len(parsedBeacons), timedoutbeacons)
				}
			}(parsedBeacons)
		},
	}
	rootCmd.AddCommand(killCmd)
	executeCmd := &cobra.Command{
		Use:   "execute [flags] command [arguments]",
		Short: "Execute a program on the remote system",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)
			//app.Printf("#\n#%s\n####", strings.Join(args, " "))

			output := cmd.Flag("output").Changed
			stdout := cmd.Flag("stdout").Value.String()
			stderr := cmd.Flag("stderr").Value.String()
			saveLoot := cmd.Flag("loot").Changed
			saveOutput := cmd.Flag("save").Changed

			// If the user wants to loot or save the output, we have to capture it regardless of if they specified -o
			var captureOutput bool = output || saveLoot || saveOutput

			if output {
				app.Printf("Using --output in beacon mode, if the command blocks the task will never complete\n\n")
			}
			var beaconWG sync.WaitGroup
			beaconWG.Add(len(beacons))
			for _, beacon := range beacons {
				go func(beacon string) {
					_, err := client.rpc.Execute(context.Background(), &sliverpb.ExecuteReq{
						Request: &commonpb.Request{
							Async:    true,
							Timeout:  int64(60),
							BeaconID: beacon,
						},
						Path:   args[0],
						Args:   args[1:],
						Output: captureOutput,
						Stderr: stderr,
						Stdout: stdout,
					})
					if err != nil {
						app.Printf("%s\n\n", err)
						beaconWG.Done()
						return
					}
					//app.Printf("%s\n", exec.Response.TaskID)
					beaconWG.Done()
				}(beacon)
			}
			beaconWG.Wait()
			app.Printf("Command \"%s\" sent to %d beacons", strings.Join(args, " "), len(beacons))
		},
	}
	executeCmd.Flags().BoolP("output", "o", false, "capture command output")
	executeCmd.Flags().StringP("stdout", "O", "", "remote path to redirect STDOUT to")
	executeCmd.Flags().StringP("stderr", "E", "", "remote path to redirect STDERR to")
	executeCmd.Flags().BoolP("loot", "X", false, "save output as loot")
	executeCmd.Flags().BoolP("save", "s", false, "save output to a file")
	rootCmd.AddCommand(executeCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "back",
		Short: "Go back to main menu",
		Run: func(cmd *cobra.Command, args []string) {
			app.SwitchMenu("")
		},
	})
	for _, cmd := range rootCmd.Commands() {
		c := carapace.Gen(cmd)

		if cmd.Args != nil {
			c.PositionalAnyCompletion(
				carapace.ActionCallback(func(c carapace.Context) carapace.Action {
					return carapace.ActionFiles()
				}),
			)
		}

		flagMap := make(carapace.ActionMap)
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "file" || strings.Contains(f.Usage, "file") {
				flagMap[f.Name] = carapace.ActionFiles()
			}
		})

		c.FlagCompletion(flagMap)
	}

	rootCmd.InitDefaultHelpCmd()

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.DisableFlagsInUseLine = true
	return rootCmd
}
