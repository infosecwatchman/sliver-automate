package main

import (
	"context"
	"fmt"
	"github.com/bishopfox/sliver/client/assets"
	consts "github.com/bishopfox/sliver/client/constants"
	"github.com/bishopfox/sliver/client/transport"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"google.golang.org/grpc"
	"io"
	"log"
)

type SliverConnection struct {
	rpc rpcpb.SliverRPCClient
	ln  *grpc.ClientConn
}

func makeRequest(session *clientpb.Session) *commonpb.Request {
	if session == nil {
		return nil
	}
	timeout := int64(60)
	return &commonpb.Request{
		SessionID: session.ID,
		Timeout:   timeout,
	}
}

func (con *SliverConnection) sliverConnect(configPath string) {
	var config *assets.ClientConfig
	var err error
	config = selectConfig()
	if config == nil {
		fmt.Println("Config not found in default location, using \"-config\" flag.")
		// load the client configuration from the filesystem
		config, err = assets.ReadConfig(configPath)
		if err != nil {
			log.Fatal(err)
		}
		//log.Fatal("config file not found.")
	} else {
		fmt.Printf("Config found in default location.")
	}
	// connect to the server
	rpc, ln, err := transport.MTLSConnect(config)
	if err != nil {
		log.Fatal(err)
	}
	con.rpc = rpc
	con.ln = ln
	log.Printf("[*] Connected to sliver server: %s:%d", config.LHost, config.LPort)
	//defer ln.Close()
	// Open the event stream to be able to collect all events sent by  the server
	eventStream, err := con.rpc.Events(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		err = func() error {
			// infinite loop
			for {
				event, err := eventStream.Recv()
				if err == io.EOF || event == nil {
					return err
				}
				switch event.EventType {
				case consts.BeaconRegisteredEvent:
					//idregex, _ := regexp.Compile(`([0-9a-c].{7}-[0-9a-c].{3}-[0-9a-c].{3}-[0-9a-c].{3}-[0-9a-c].{11})`)
					//idmatch := idregex.FindSubmatch(event.Data)
					beaconID := event.Data[2:38]
					app.Printf("Beacon registered: %s\n", string(beaconID))
					//app.Printf(string(event.Data))
					//client.rpc.Cd(context.Background(), *sliverpb.CdReq{Request: makeRequest(string(idmatch))})
				case consts.BeaconTaskResultEvent:
					app.Printf("task finished.")

				default:
					app.Printf("[*] - %s", event.EventType)
				}

				/*
					// Trigger event based on type
					switch event.EventType {
					// a new session just came in
					case consts.SessionOpenedEvent:
						session := event.Session
						// call any RPC you want, for the full list, see
						// https://github.com/BishopFox/sliver/blob/master/protobuf/rpcpb/services.proto
						resp, err := rpc.Execute(context.Background(), &sliverpb.ExecuteReq{
							Path:    `c:\windows\system32\calc.exe`,
							Output:  false,
							Request: makeRequest(session),
						})
						if err != nil {
							log.Fatal(err)
						}
						rpc.Cd(context.Background(), &sliverpb.CdReq{
							Path:    "",
							Request: makeRequest(),
						})
						// Don't forget to check for errors in the Response object
						if resp.Response != nil && resp.Response.Err != "" {
							log.Fatal(resp.Response.Err)
						}
					}

				*/

			}
		}()
		if err != nil {
			log.Fatal(err)
		}
	}()
}
