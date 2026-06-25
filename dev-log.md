6-24-26

parseHeader reads the first 12 bytes of a DNS packet.
Each DNS header field is 2 bytes.
I reuse readU16 at offsets 0, 2, 4, 6, 8, 10.
The function returns an error if the packet is shorter than 12 bytes.