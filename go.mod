module github.com/farhansabbir/telnet

go 1.23.5

// require (
// 	github.com/farhansabbir/go-ping v1.0.6
// 	golang.org/x/net v0.34.0 // indirect
// )

require github.com/farhansabbir/go-ping v1.0.6

require (
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
)

replace github.com/farhansabbir/go-ping => ./go-ping/
