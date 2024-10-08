package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/LockBlock-dev/MinePot/handler"
	"github.com/LockBlock-dev/MinePot/types"
	"github.com/LockBlock-dev/MinePot/util"
	"github.com/Tnze/go-mc/net"
	"github.com/mailgun/proxyproto"
	"github.com/muesli/cache2go"
	"github.com/speedata/optionparser"
)

func main() {
	var configPath string = "/etc/minepot/config.json"

	parser := optionparser.NewOptionParser()

	parser.On("-c", "--config <path>", "Path to the config file", &configPath)

	err := parser.Parse()
	if err != nil {
		fmt.Println(err)
		fmt.Println("\"MinePot -h --help\" for help")
		return
	}

	config, err := util.GetConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	var file *os.File

	if config.WriteLogs {
		// Open logs file
		file, err = os.OpenFile(config.LogFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644) // 644 = rw-,r--,r--
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}

	if config.WriteHistory {
		// Open history file
		historyFile, err := os.OpenFile(config.HistoryFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644) // 644 = rw-,r--,r--
		if err != nil {
			log.Fatal(err)
		}
		defer historyFile.Close()

		_, err = historyFile.WriteString("datetime, ip, packets_count, reported, handshake, ping\n")
		if err != nil {
			log.Fatal("Failed to write history headers:", err)
		}
	}

	if config.Haproxy {
		log.Println("Using HAProxy protocol, make sure to configure the HAProxy to use it!")
	}

	// Setup the cache
	_ = cache2go.Cache("MinePot")

	// Listen for incoming connections on TCP port X (see config.json)
	address := fmt.Sprintf(":%d", config.Port)
	listener, err := net.ListenMC(address)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		listener.Close()
	}()

	log.Printf("Server listening on port %d\nYou can edit the config at %s", config.Port, configPath)

	if config.WriteLogs {
		// Logs the logs file path
		cwd, err := os.Getwd()
		if err == nil {
			log.Println("Find the logs at: " + path.Join(cwd, config.LogFile))
		}

		// Setup logs to a file
		log.SetOutput(file)
	}

	for {
		// Wait for a client to connect
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			return
		}

		// Set a timeout of X seconds (see config.json)
		conn.Socket.SetDeadline(time.Now().Add(time.Duration(config.IdleTimeoutS) * time.Second))

		srcAddr := conn.Socket.RemoteAddr()
		DestAddr := conn.Socket.LocalAddr()
		if config.Haproxy {

			h, err := proxyproto.ReadHeader(conn)
			if err != nil {
				log.Fatal("Client is not using the PROXY protocol " + srcAddr.String())
				conn.Close()
				continue
			}

			if h.IsLocal {
				conn.Close()
				continue
			}

			srcAddr = h.Source
			DestAddr = h.Destination
		}

		connWrapper := types.ConnWrapper{
			Conn:     conn,
			SrcAddr:  srcAddr,
			DestAddr: DestAddr,
			Config:   config,
		}

		// Start a new goroutine to handle the connection
		go handler.HandleConnection(connWrapper)
	}
}
