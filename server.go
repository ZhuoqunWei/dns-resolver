package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

func handlePacket(packet []byte, records map[string][4]byte) (Message, []byte, error) {
	msg, err := parseMessage(packet)
	if err != nil {
		return Message{}, nil, fmt.Errorf("parse message: %w", err)
	}

	response, err := buildResponse(msg, records)
	if err != nil {
		return msg, nil, fmt.Errorf("build response: %w", err)
	}

	return msg, response, nil
}

func serveUDP(conn *net.UDPConn, records map[string][4]byte, output io.Writer) error {
	buf := make([]byte, 512)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			fmt.Fprintln(output, "read error:", err)
			continue
		}

		msg, response, err := handlePacket(buf[:n], records)
		if err != nil {
			fmt.Fprintln(output, "query error from", remoteAddr, ":", err)
			continue
		}

		responseStatus := "empty response"
		if binary.BigEndian.Uint16(response[6:8]) > 0 {
			responseStatus = "answer returned"
		}

		fmt.Fprintln(output, "----- DNS Query -----")
		fmt.Fprintln(output, "From:", remoteAddr)
		fmt.Fprintln(output, "Bytes received:", n)
		fmt.Fprintf(output, "ID: 0x%04x\n", msg.Header.ID)
		fmt.Fprintln(output, "Question:", msg.Question.Name)
		fmt.Fprintln(output, "QType:", msg.Question.QType)
		fmt.Fprintln(output, "QClass:", msg.Question.QClass)
		fmt.Fprintln(output, "Response:", responseStatus)

		if _, err := conn.WriteToUDP(response, remoteAddr); err != nil {
			fmt.Fprintln(output, "write error:", err)
		}
	}
}
