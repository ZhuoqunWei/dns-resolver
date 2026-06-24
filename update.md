Daily DNS Update: June 22

Task: initialize project, wirte a simple byte array manipulation program
What I changed: initialize the project; byte array program
Command I ran: go run .
Output:
Space-separated Hex representation:
12 34
Decimal representation:
[18 52]
Combined number: 4660

Commit message:
➜  dns-resolver git:(master) ✗ git add dev-log.md go.mod main.go README.md 
➜  dns-resolver git:(master) ✗ git commit -m "project setup/bit manipulation"
[master (root-commit) 38215e0] project setup/bit manipulation
 4 files changed, 20 insertions(+)
 create mode 100644 README.md
 create mode 100644 dev-log.md
 create mode 100644 go.mod
 create mode 100644 main.go
What I understand:
how to compile and run code in go; how to print and do simple bit manipulation in go
What I’m unsure about:
why we are doing this exercise
Next task:
Unknown

Daily DNS Update: June 23

Yesterday’s progress:
Completed DNS-001: project setup and byte manipulation practice.

Today’s one task:
Implement DNS-002 readU16(data []byte, offset int) (uint16, error)

Definition of done:

readU16([]byte{0x12, 0x34}, 0) returns 4660
readU16([]byte{0x00, 0x01}, 0) returns 1
readU16([]byte{0x12}, 0) returns an error
readU16([]byte{0x12, 0x34, 0x56}, 1) returns 13398
readU16([]byte{0x00, 0x01}, -1) returns error
readU16([]byte{0x12, 0x34}, 2) returns an error
go test passes
commit is made

Blocker:
No blocker right now. I hit a test failure because the function was slicing bytes before checking the length, but I understand the issue now and am fixing it by moving validation before indexing/slicing.

Commit made? Not yet

Commit message or link:
Not yet.