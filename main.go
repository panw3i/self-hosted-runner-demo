package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Printf("Hello from self-hosted runner demo!\n")
	fmt.Printf("OS:   %s\n", runtime.GOOS)
	fmt.Printf("Arch: %s\n", runtime.GOARCH)
}
