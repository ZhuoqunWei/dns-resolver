package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	const listenAddress = "127.0.0.1:8053"

	zone, err := loadZone("records.json")
	if err != nil {
		log.Fatal("load zone: ", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer udpConn.Close()

	tcpListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	defer tcpListener.Close()

	fmt.Println("DNS server listening on 127.0.0.1:8053 over UDP and TCP")

	serverErrors := make(chan error, 2)
	go func() {
		serverErrors <- serveUDP(udpConn, zone, os.Stdout)
	}()
	go func() {
		serverErrors <- serveTCP(tcpListener, zone, os.Stdout)
	}()

	if err := <-serverErrors; err != nil {
		log.Print(err)
	}
}
