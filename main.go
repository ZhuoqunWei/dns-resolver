package main

import "fmt"

func main() {
	data := []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: recursion desired
		0x00, 0x01, // QDCOUNT: 1 question
		0x00, 0x00, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT

		// QNAME:
		0x03, 'w', 'w', 'w',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE and QCLASS
		0x00, 0x01, // QTYPE: A
		0x00, 0x01, // QCLASS: IN
	}

	msg, err := parseMessage(data)
	if err != nil {
		fmt.Println("parse error:", err)
		return
	}

	fmt.Printf("ID: 0x%04x\n", msg.Header.ID)
	fmt.Printf("RD: %t\n", msg.Flags.RD)
	fmt.Printf("Question: %s\n", msg.Question.Name)
	fmt.Printf("QType: %d\n", msg.Question.QType)
	fmt.Printf("QClass: %d\n", msg.Question.QClass)
}
