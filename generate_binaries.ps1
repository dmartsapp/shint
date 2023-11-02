foreach ($os in @("windows","linux","darwin")){
    foreach($arch in @("arm64","amd64")){
        $Env:GOOS="$os"; $Env:GOARCH="$arch"; go build -o bin/telnet.$os.$arch main.go
    }
}