[![Go Reference](https://pkg.go.dev/badge/github.com/infosecwatchman/sliver-automate/client.svg)](https://pkg.go.dev/github.com/infosecwatchman/sliver-automate/client)
### Sliver-automate 
 
This tool is designed to be a custom client to be able to select or "search" multiple beacons or sessions and run commands across all of them simultanously. Also, setup "triggers" to run commands on newly connected beacons. 

It is a "console" style tool with auto-completion and "-h" syntax help.

#### Overview

This utility provides expanded usability with sliver, and adds quality of life features when trying to manage many beacons.

By default, when the binary runs it will look for a sliver config directory on your local system, and use it if it's there. 

You can also specify a sliver config file with `-config` flag.
```
$ sliver_automate -h
Usage of sliver_automate_windows:
  -config string
```
There are a number of commands and subcommands with in the CLI. 
```
sliver-automate > help
Usage:
   [command]

Available Commands:
  exit        Exit sliver-automate console.
  help        Help about any command
  interact    Use interact to perform actions on multiple beacons simultaneously.
  list        List all active Beacons
  search      Search for beacons with given regex. Useful for testing filters or regex matches.
  trigger     List, add, import, or export, triggers based on existing impant profiles.

Flags:
  -h, --help   help for this command

Use " [command] --help" for more information about a command. 
sliver-automate >
```
