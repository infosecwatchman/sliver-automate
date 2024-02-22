package main

import (
	"regexp"
	"strings"
	"sync"
)

func prettifyDownloadName(path string) string {
	nonAlphaNumericRegex, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		// Well, we tried.
		return path
	}

	pathNoSeparators := strings.ReplaceAll(path, "\\", "_")
	pathNoSeparators = strings.ReplaceAll(pathNoSeparators, "/", "_")

	filteredString := nonAlphaNumericRegex.ReplaceAllString(pathNoSeparators, "_")

	// Collapse multiple underscores into one
	multipleUnderscoreRegex, err := regexp.Compile("_{2,}")
	if err != nil {
		return filteredString
	}

	filteredString = multipleUnderscoreRegex.ReplaceAllString(filteredString, "_")

	// If there is an underscore at the front of the filename, strip that off
	if strings.HasPrefix(filteredString, "_") {
		filteredString = filteredString[1:]
	}

	return filteredString
}

func AsyncBeacons(command func(beacon string) error, beacons []string) {
	var beaconWG sync.WaitGroup
	beaconWG.Add(len(beacons))
	for _, beacon := range beacons {
		go func(beacon string) {
			err := command(beacon)
			if err != nil {
				app.Printf("%s\n\n", err)
				beaconWG.Done()
				return
			}
			beaconWG.Done()
		}(beacon)
	}
	beaconWG.Wait()
}
