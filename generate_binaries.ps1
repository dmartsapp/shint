foreach ($os in @("windows","linux","darwin")){
    foreach($arch in @("arm64","amd64")){
        $Env:GOOS="$os"; $Env:GOARCH="$arch"; $Env:CGO_ENABLED=0; go build -buildvcs=true -ldflags="-s -w" -o bin/telnet.$os.$arch
    }
}