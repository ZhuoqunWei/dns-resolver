package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

func main() {

	// UDP server
	// 1. Choose the address the UDP server will listen on
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8053")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Start listening on UDP
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Println("DNS UDP server listening on 127.0.0.1:8053")

	// 3. Create a buffer for incoming packets
	buf := make([]byte, 512)
	records := map[string][4]byte{
		"example.com": {1, 2, 3, 4},
		"test.local":  {5, 6, 7, 8},
	}

	// 4. Keep server running forever
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("read error:", err)
			continue
		}
		packet := buf[:n]

		msg, err := parseMessage(packet)
		if err != nil {
			fmt.Println("parse error from", remoteAddr, ":", err)
			continue
		}

		response, err := buildResponse(msg, records)
		if err != nil {
			fmt.Println("response build error:", err)
			continue
		}

		responseStatus := "empty response"
		if binary.BigEndian.Uint16(response[6:8]) > 0 {
			responseStatus = "answer returned"
		}

		fmt.Println("----- DNS Query -----")
		fmt.Println("From:", remoteAddr)
		fmt.Println("Bytes received:", n)
		fmt.Printf("ID: 0x%04x\n", msg.Header.ID)
		fmt.Printf("Question: %s\n", msg.Question.Name)
		fmt.Printf("QType: %d\n", msg.Question.QType)
		fmt.Printf("QClass: %d\n", msg.Question.QClass)
		fmt.Println("Response:", responseStatus)

		_, err = conn.WriteToUDP(response, remoteAddr)
		if err != nil {
			fmt.Println("write error:", err)
			continue
		}
	}
}
