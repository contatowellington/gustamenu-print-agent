//go:build !windows

package main

import "fmt"

func runApp() {
	fmt.Println("Este aplicativo só funciona no Windows.")
}
