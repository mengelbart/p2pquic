package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/mengelbart/p2pquic"
)

const message = "foobar"

var runAsServer bool
var ice bool
var addr string
var udpPort int
var localPort int
var remotePort int

func main() {
	flag.BoolVar(&runAsServer, "server", false, "run as server as oppposed to client which is the default")
	flag.BoolVar(&ice, "ice", false, "Run over ICE")
	flag.StringVar(&addr, "addr", "localhost", "address")
	flag.IntVar(&udpPort, "port", 4242, "UDP Port")
	flag.IntVar(&localPort, "local", 9000, "ICE signaling server local port")
	flag.IntVar(&remotePort, "remote", 9001, "ICE signaling server remote port")

	flag.Parse()

	if runAsServer {
		if err := server(); err != nil {
			log.Fatal(err)
		}
	}

	if err := client(); err != nil {
		log.Fatal(err)
	}
}

func server() error {
	s, err := p2pquic.NewServer()
	if err != nil {
		return err
	}
	if ice {
		return s.RunICE(localPort, remotePort)
	}
	return s.RunUDP(fmt.Sprintf("%v:%v", addr, udpPort))
}

func client() error {
	c, err := p2pquic.NewClient(message)
	if err != nil {
		return err
	}
	if ice {
		return c.RunICE(localPort, remotePort)
	}
	return c.RunUDP(fmt.Sprintf("%v:%v", addr, udpPort))
}
