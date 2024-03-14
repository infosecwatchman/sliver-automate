package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bishopfox/sliver/client/console"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"github.com/tjarratt/babble"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
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
		Short: "Search for beacons with given regex. Useful for testing filters or regex matches.",
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
			}
		},
	}
	triggerCmd := &cobra.Command{
		Use: "trigger",
	}
	triggerAddCmd := &cobra.Command{
		Use:   "add [implant name]",
		Short: "Add another beacon when new beacon matching implant name is registered. (persistence)",
		Long: `When a newly registered beacon matches, a new implant profile is created 
matching the new beacon and appends a random string to the name to prevent infinite beacon creation.
The beacon interval and jitter is tripled with the new implant profile to facilitate 'long hall' callbacks.`,
		Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			builds, err := client.rpc.ImplantBuilds(context.Background(), &commonpb.Empty{})
			if err != nil {
				app.Printf("%s\n\n", err)
			}
			//app.Printf("%s\n\n", builds.String())
			bconfigs := builds.GetConfigs()
			if len(bconfigs) == 0 {
				app.Printf("No implants found.\n")
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var implants []string
			for sliverName, _ := range bconfigs {
				implants = append(implants, sliverName)
			}

			return implants, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			reader := bufio.NewReader(os.Stdin)
			app.Printf("The default trigger has terrible OpSec, are you sure you want to create a trigger for %s? (Y/y, Ctrl-C):\n", args[0])
			text, _ := reader.ReadString('\n')
			answer := strings.TrimSpace(text)

			if (answer == "Y") || (answer == "y") {

				builds, err := client.rpc.ImplantBuilds(context.Background(), &commonpb.Empty{})
				if err != nil {
					app.Printf("%s\n\n", err)
				}
				//app.Printf("%s\n\n", builds.String())
				bconfigs := builds.GetConfigs()
				babbler := babble.NewBabbler()
				now := strings.Split(strconv.Itoa(int(time.Now().Unix())), "")
				randomName := fmt.Sprintf("%s_%s", babbler.Babble(), strings.Join(now[len(now)-4:], ""))
				if len(bconfigs) == 0 {
					app.Printf("No implants found.\n")
				}
				for sliverName, config := range bconfigs {
					if sliverName == args[0] {
						newconfig := config
						newconfig.BeaconJitter = config.BeaconJitter/2 + config.BeaconJitter
						newconfig.BeaconInterval = config.BeaconInterval * 3
						newconfig.Name = fmt.Sprintf("%s", randomName)
						start := time.Now()
						ctrl := make(chan bool)

						go Until(fmt.Sprintf("Compiling %s, please wait ... ", config.Name), ctrl)

						file, err := client.rpc.Generate(context.Background(), &clientpb.GenerateReq{Config: newconfig})
						if err != nil {
							app.Printf("%s\n\n", err)
						}
						//app.Printf("%s\n\n", file)
						ctrl <- true
						<-ctrl

						elapsed := time.Since(start)
						app.Printf("Build completed in %s\n", elapsed.Round(time.Second))

						err = writeFile(file.File.Name, file.File.Data, os.FileMode(0755))
						if err != nil {
							app.Printf("%s\n", err)
							app.Printf("Trigger not created.\n")
							return
						}
						triggers = append(triggers, trigger{
							ImplantName:         newconfig.Name,
							ParentImplantName:   sliverName,
							NewBeaconJitter:     newconfig.BeaconJitter,
							NewBeaconInterval:   newconfig.BeaconInterval,
							ParentImplantConfig: config,
							Filename:            file.File.Name,
						})
					}
				}
			}
		},
	}
	listTriggersCmd := &cobra.Command{
		Use:   "list",
		Short: "Lists any active triggers",
		Run: func(cmd *cobra.Command, args []string) {
			tbl := table.New("Implant Trigger", "New Interval", "New Jitter", "New Implant Name")
			if len(triggers) == 0 {
				app.Printf("No triggers defined. \n")
			} else {
				for _, triggerIter := range triggers {
					tbl.AddRow(triggerIter.ParentImplantName, triggerIter.NewBeaconInterval, triggerIter.NewBeaconJitter, triggerIter.ImplantName)
				}
				tbl.Print()
				app.Printf("\n")
			}
		},
	}
	triggerImportCmd := &cobra.Command{
		Use:   "import",
		Short: "Import saved triggers into current session.",
		Run: func(cmd *cobra.Command, args []string) {
			if _, err := os.Stat(cmd.Flag("filename").Value.String()); errors.Is(err, os.ErrNotExist) {
				app.Printf("%s does not exist.\n", cmd.Flag("filename").Value.String())
				return
			}
			var importTriggers []trigger
			triggerFile, err := os.ReadFile(cmd.Flag("filename").Value.String())
			if err != nil {
				app.Printf("%s\n", err)
				return
			}
			err = json.Unmarshal(triggerFile, &importTriggers)
			if err != nil {
				app.Printf("%s\n", err)
				return
			}
			builds, err := client.rpc.ImplantBuilds(context.Background(), &commonpb.Empty{})
			if err != nil {
				app.Printf("%s\n\n", err)
			}
			bconfigs := builds.GetConfigs()
			if len(bconfigs) == 0 {
				app.Printf("No implants found.\n")
			}
			var implants []string
			for sliverName, _ := range bconfigs {
				implants = append(implants, sliverName)
			}
			for _, triggerIter := range importTriggers {
				if slices.Contains(implants, triggerIter.ParentImplantName) {
					if slices.Contains(implants, triggerIter.ImplantName) {
						if _, err := os.Stat(triggerIter.Filename); errors.Is(err, os.ErrNotExist) {
							app.Printf("%s does not exist. Redownloading implant...\n", triggerIter.Filename)
							implantProfile, err := client.rpc.Regenerate(context.Background(), &clientpb.RegenerateReq{ImplantName: triggerIter.ImplantName})
							if err != nil {
								app.Printf("Unable to download implant %s: %s\n", triggerIter.ImplantName, err)
							}
							err = writeFile(triggerIter.Filename, implantProfile.GetFile().Data, os.FileMode(0755))
							if err != nil {
								app.Printf("%s\n", err)
							}
						}
						triggers = append(triggers, triggerIter)
					} else {
						app.Printf("Unable to import trigger, implant profile %s does not exist.\n", triggerIter.ImplantName)
					}
				} else {
					app.Printf("Unable to import trigger, implant profile %s does not exist.\n", triggerIter.ParentImplantName)
				}
			}

		},
	}
	triggerExportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export currently defined triggers.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(triggers) == 0 {
				app.Printf("No triggers defined. \n")
			} else {
				triggerJson, err := json.Marshal(triggers)
				if err != nil {
					app.Printf("%s\n", err)
				}
				err = writeFile(cmd.Flag("filename").Value.String(), triggerJson, os.FileMode(0666))
				if err != nil {
					app.Printf("%s\n", err)
				}
			}
		},
	}
	triggerImportCmd.Flags().StringP("filename", "f", "triggers.json", "Filename to import the triggers as. Defaults to triggers.json")
	triggerExportCmd.Flags().StringP("filename", "f", "triggers.json", "Filename to export the triggers as. Defaults to triggers.json")
	triggerCmd.AddCommand(triggerExportCmd)
	triggerCmd.AddCommand(triggerImportCmd)
	triggerCmd.AddCommand(listTriggersCmd)
	triggerCmd.AddCommand(triggerAddCmd)
	triggerAddCmd.DisableFlagsInUseLine = true
	triggerAddCmd.CompletionOptions.DisableDefaultCmd = false
	rootCmd.AddCommand(triggerCmd)
	interactCmd := &cobra.Command{
		Use:   "interact",
		Short: "Use interact to perform actions on multiple beacons simultaneously.",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			app.Printf("Must select `beacon`.")
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
	rootCmd.AddCommand(interactCmd)
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(listCmd)
	/*
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
		rootCmd.CompletionOptions.DisableDefaultCmd = true
		rootCmd.DisableFlagsInUseLine = true
	*/
	rootCmd.InitDefaultHelpCmd()
	return rootCmd
}
