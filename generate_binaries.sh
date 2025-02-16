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
            exit 1
        fi
else
    echo "Binary directory exists. Generating binaries"
fi
declare -a OSES=("linux" "darwin" "windows")
declare -a ARCHS=("arm64" "amd64")
for GOOS in "${OSES[@]}"
do
    for ARCH in "${ARCHS[@]}"
    do
        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$ARCH go build -buildvcs=true -ldflags="-s -w" -o bin/telnet.$GOOS.$ARCH
        if [ $? -eq 0 ]
        then
            echo "Created $GOOS $ARCH binary"
        else
            echo "Unable to generate the $GOOS $ARCH binary"
        fi
    done
    
done
