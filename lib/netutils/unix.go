//go:build !windows && (arm64 || amd64)
// +build !windows
// +build arm64 amd64

// @Description: For UNIX
// @File:  unix.go
// @Author: github.com/farhansabbir
// @Date: 2024-05-12 22:28
package netutils

import "fmt"

func sendReq() {
	fmt.Println("UNIX")
}
