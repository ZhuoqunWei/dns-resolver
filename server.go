package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	tcpIdleTimeout  = 10 * time.Second
	tcpWriteTimeout = 5 * time.Second
)

var dnsLogMu sync.Mutex

func handlePacket(packet []byte, zone Zone) (Message, []byte, error) {
	msg, err := parseMessage(packet)
	if err != nil {
		return Message{}, nil, fmt.Errorf("parse message: %w", err)
	}

	return buildPacketResponse(msg, zone, udpResponseSizeLimit(msg))
}

func handlePacketWithLimit(packet []byte, zone Zone, responseLimit int) (Message, []byte, error) {
	msg, err := parseMessage(packet)
	if err != nil {
		return Message{}, nil, fmt.Errorf("parse message: %w", err)
	}

	return buildPacketResponse(msg, zone, responseLimit)
}

func buildPacketResponse(msg Message, zone Zone, responseLimit int) (Message, []byte, error) {
	response, err := buildResponseWithLimit(msg, zone, responseLimit)
	if err != nil {
		return msg, nil, fmt.Errorf("build response: %w", err)
	}

	return msg, response, nil
}

func udpResponseSizeLimit(msg Message) int {
	if msg.EDNS == nil {
		return maxDNSUDPMessageSize
	}

	advertised := int(msg.EDNS.UDPSize)
	if advertised < maxDNSUDPMessageSize {
		return maxDNSUDPMessageSize
	}
	if advertised > maxEDNSUDPMessageSize {
		return maxEDNSUDPMessageSize
	}

	return advertised
}

func serveUDP(conn *net.UDPConn, zone Zone, output io.Writer) error {
	buf := make([]byte, maxEDNSUDPMessageSize)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			fmt.Fprintln(output, "read error:", err)
			continue
		}

		msg, response, err := handlePacket(buf[:n], zone)
		if err != nil {
			fmt.Fprintln(output, "query error from", remoteAddr, ":", err)
			continue
		}

		logDNSQuery(output, "UDP", remoteAddr, n, msg, response)

		if _, err := conn.WriteToUDP(response, remoteAddr); err != nil {
			fmt.Fprintln(output, "write error:", err)
		}
	}
}

func serveTCP(listener net.Listener, zone Zone, output io.Writer) error {
	var connections sync.Map
	var handlers sync.WaitGroup

	defer func() {
		connections.Range(func(key, _ any) bool {
			_ = key.(net.Conn).Close()
			return true
		})
		handlers.Wait()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept TCP connection: %w", err)
		}

		connections.Store(conn, struct{}{})
		handlers.Add(1)
		go func(conn net.Conn) {
			defer handlers.Done()
			defer connections.Delete(conn)
			defer conn.Close()

			if err := handleTCPConnection(conn, zone, output); err != nil {
				fmt.Fprintln(output, "TCP connection error from", conn.RemoteAddr(), ":", err)
			}
		}(conn)
	}
}

func handleTCPConnection(conn net.Conn, zone Zone, output io.Writer) error {
	return handleTCPConnectionWithTimeouts(
		conn,
		zone,
		output,
		tcpIdleTimeout,
		tcpWriteTimeout,
	)
}

func handleTCPConnectionWithTimeouts(
	conn net.Conn,
	zone Zone,
	output io.Writer,
	readTimeout time.Duration,
	writeTimeout time.Duration,
) error {
	if readTimeout <= 0 {
		return fmt.Errorf("TCP read timeout must be positive")
	}
	if writeTimeout <= 0 {
		return fmt.Errorf("TCP write timeout must be positive")
	}

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return fmt.Errorf("set TCP read deadline: %w", err)
		}

		packet, err := readTCPMessage(conn)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read DNS message: %w", err)
		}

		msg, response, err := handlePacketWithLimit(packet, zone, maxDNSTCPMessageSize)
		if err != nil {
			return err
		}

		logDNSQuery(output, "TCP", conn.RemoteAddr(), len(packet), msg, response)

		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			return fmt.Errorf("set TCP write deadline: %w", err)
		}

		if err := writeTCPMessage(conn, response); err != nil {
			return fmt.Errorf("write DNS message: %w", err)
		}
	}
}

func readTCPMessage(reader io.Reader) ([]byte, error) {
	var lengthBytes [2]byte
	if _, err := io.ReadFull(reader, lengthBytes[:]); err != nil {
		return nil, err
	}

	messageLength := int(binary.BigEndian.Uint16(lengthBytes[:]))
	if messageLength == 0 {
		return nil, fmt.Errorf("DNS message length must be greater than zero")
	}

	message := make([]byte, messageLength)
	if _, err := io.ReadFull(reader, message); err != nil {
		return nil, err
	}

	return message, nil
}

func writeTCPMessage(writer io.Writer, message []byte) error {
	if len(message) == 0 {
		return fmt.Errorf("DNS message must not be empty")
	}
	if len(message) > maxDNSTCPMessageSize {
		return fmt.Errorf("DNS message length %d exceeds TCP limit %d", len(message), maxDNSTCPMessageSize)
	}

	frame := make([]byte, 2+len(message))
	binary.BigEndian.PutUint16(frame[0:2], uint16(len(message)))
	copy(frame[2:], message)

	for len(frame) > 0 {
		n, err := writer.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}

	return nil
}

func logDNSQuery(
	output io.Writer,
	transport string,
	remoteAddr net.Addr,
	bytesReceived int,
	msg Message,
	response []byte,
) {
	responseStatus := "empty response"
	if binary.BigEndian.Uint16(response[6:8]) > 0 {
		responseStatus = "answer returned"
	}

	var entry bytes.Buffer
	fmt.Fprintln(&entry, "----- DNS Query -----")
	fmt.Fprintln(&entry, "Transport:", transport)
	fmt.Fprintln(&entry, "From:", remoteAddr)
	fmt.Fprintln(&entry, "Bytes received:", bytesReceived)
	fmt.Fprintf(&entry, "ID: 0x%04x\n", msg.Header.ID)
	fmt.Fprintln(&entry, "Question:", msg.Question.Name)
	fmt.Fprintln(&entry, "QType:", msg.Question.QType)
	fmt.Fprintln(&entry, "QClass:", msg.Question.QClass)
	fmt.Fprintln(&entry, "Response:", responseStatus)

	dnsLogMu.Lock()
	defer dnsLogMu.Unlock()
	_, _ = output.Write(entry.Bytes())
}
