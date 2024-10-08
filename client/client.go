package client

import (
	"context"
	"flag"
	"fmt"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/reeflective/console"
	"github.com/reeflective/readline"
	"log"
	"os"
)

var (
	client   SliverConnection
	app      *console.Console
	ctx      context.Context
	triggers []Trigger
)

type Trigger struct {
	ImplantName         string
	ParentImplantName   string
	NewBeaconJitter     int64
	NewBeaconInterval   int64
	ParentImplantConfig *clientpb.ImplantConfig
	Filename            string
	triggered           []TriggeredType
}

type TriggeredType struct {
	Init           bool
	ParentBeaconID string
	uploadcount    int
}

func Interact() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var configPath string
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.StringVar(&configPath, "config", "", "path to sliver client config file")
	flagset.Parse(os.Args[1:])
	app = console.New("sliver-automate")
	app.SetPrintLogo(func(_ *console.Console) {
		fmt.Print(`
  _________.__  .__                        _____          __                         __          
 /   _____/|  | |__|__  __ ___________    /  _  \  __ ___/  |_  ____   _____ _____ _/  |_  ____  
 \_____  \ |  | |  \  \/ // __ \_  __ \  /  /_\  \|  |  \   __\/  _ \ /     \\__  \\   __\/ __ \ 
 /        \|  |_|  |\   /\  ___/|  | \/ /    |    \  |  /|  | (  <_> )  Y Y  \/ __ \|  | \  ___/ 
/_______  /|____/__| \_/  \___  >__|    \____|__  /____/ |__|  \____/|__|_|  (____  /__|  \___  >
        \/                    \/                \/                         \/     \/          \/
`)
	})

	mainMenu := app.ActiveMenu()
	interactMenu := app.NewMenu("interact")
	interactMenu.AddInterrupt(readline.ErrInterrupt, returnToMain)
	mainMenu.AddInterrupt(readline.ErrInterrupt, exitConsole)
	mainMenu.AddHistorySourceFile("history", ".history")
	interactMenu.AddHistorySourceFile("history", ".history")
	mainMenu.SetCommands(MenuCommands)
	interactMenu.SetCommands(InteractBeaconCommands)
	client.SliverConnect(configPath)
	apperr := app.Start()
	if apperr != nil {
		log.Fatal(apperr)
	}

}

func returnToMain(_ *console.Console) {
	app.SwitchMenu("")
}

func exitConsole(_ *console.Console) {
	os.Exit(0)
}
