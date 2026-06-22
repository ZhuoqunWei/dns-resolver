package main

import "fmt"

func main() {
	hexData := []byte{0x12, 0x34}

	fmt.Println("\nSpace-separated Hex representation:")
	fmt.Printf("% x\n", hexData)

	fmt.Println("Decimal representation:")
	fmt.Println(hexData)

	// combine them into a single number
	combined := uint16(hexData[0])<<8 | uint16(hexData[1])
	fmt.Printf("Combined number: %d\n", combined)
}
