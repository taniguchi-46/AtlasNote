//go:build !windows

package main

import "fmt"

func reportMaintenanceFailurePlatform(message string) {
	fmt.Println(message)
}
