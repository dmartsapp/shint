module github.com/dmartsapp/telnet

go 1.24.0

// require (
// 	github.com/farhansabbir/go-ping v1.0.6
// 	golang.org/x/net v0.34.0 // indirect
// )

require (
	github.com/dmartsapp/go-ping v1.1.0
	// github.com/dmartsapp/telnet v1.8.0
	github.com/spf13/cobra v1.10.1
	github.com/spf13/pflag v1.0.10 // indirect
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
)

replace github.com/dmartsapp/go-ping => ./go-ping/
