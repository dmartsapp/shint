#!/bin/bash
if [ ! -e "bin/" ]
    then
        echo "Creating binary directory"
        mkdir bin
        if [ $? -eq 0 ]
        then
            echo "Binary directory created. Generating binaries"
        else
            echo "Could not create binary directory"
        fi
else
    echo "Binary directory exists. Generating binaries"
fi        
GOOS=linux GOARCH=amd64 go build -o bin/linux.x64 main.go
if [ $? -eq 0 ]
then
    echo "Created linux x64 binary"
else
    echo "Unable to generate the linux x64 binary"
fi
GOOS=linux GOARCH=arm64 go build -o bin/linux.arm64 main.go
if [ $? -eq 0 ]
then
    echo "Created linux arm binary"
else
    echo "Unable to generate the linux arm64 binary"
fi
GOOS=windows GOARCH=amd64 go build -o bin/windows.x64 main.go
if [ $? -eq 0 ]
then
    echo "Created windows x64 binary"
else
    echo "Unable to generate the linux x64 binary"
fi
GOOS=darwin GOARCH=amd64 go build -o bin/macos.x64 main.go
if [ $? -eq 0 ]
then
    echo "Created MacOS x64 binary"
else
    echo "Unable to generate the MacOS x64 binary"
fi
GOOS=darwin GOARCH=arm64 go build -o bin/macos.arm64 main.go
if [ $? -eq 0 ]
then
    echo "Created MacOS arm64 binary"
else
    echo "Unable to generate the MacOS arm64 binary"
fi