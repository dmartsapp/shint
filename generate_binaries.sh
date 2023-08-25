#!/bin/bash
if [ ! -e "bin/" ]
    then
        mkdir bin
fi        
GOOS=linux GOARCH=amd64 go build -o bin/linux.x64 main.go
GOOS=linux GOARCH=arm64 go build -o bin/linux.arm64 main.go
GOOS=windows GOARCH=amd64 go build -o bin/windows.x64 main.go
GOOS=darwin GOARCH=amd64 go build -o bin/macos.x64 main.go
GOOS=darwin GOARCH=arm64 go build -o bin/macos.arm64 main.go