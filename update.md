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

DNS-002 Final Update

Tests:
Command output:
go test ./...
git status
git add .
git commit -m "Implement safe readU16 with tests"
ok      github.com/zhuoqunwei/dns-resolver      0.290s
On branch master
Changes not staged for commit:
  (use "git add/rm <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
        deleted:    main.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
        dns.go
        dns_test.go
        go
        update.md

no changes added to commit (use "git add" and/or "git commit -a")
[master a1749d2] Implement safe readU16 with tests
 5 files changed, 116 insertions(+), 17 deletions(-)
 create mode 100644 dns.go
 create mode 100644 dns_test.go
 create mode 100644 go
 delete mode 100644 main.go
 create mode 100644 update.md
Commit message: git commit -m "Implement safe readU16 with tests"
What I learned: how to write go test and run test command. i learned go format. i learned details of printf and format verbs. i learned about how to write a function in go and main is not necessary
What I’m still unsure about: still behind on go syntax. like for loop and return value.
My guess for next task: read about what to parse on dns



Daily DNS Update: June 24

Yesterday:
Finished readU16 with tests and cleaned accidental go file.

Today:
Implement parseHeader(data []byte) (Header, error).

Done means:
12-byte packet parses into six fields, short packet returns error, go test passes, commit made.
passed test
git commit -m "implemented parseHeader to parse first 12 bytes in the header"

Blocker:
not too much, but i did use a little auto completion to get the job done
My own guess:
next step is to break down the flags and parse them