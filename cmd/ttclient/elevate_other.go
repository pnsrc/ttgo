//go:build !darwin && !windows

package main

import (
	"log"
	"os"
)

func ensureElevated() {
	if os.Geteuid() == 0 {
		return
	}
	log.Fatalln("Elevated privileges required. Please run with sudo.")
}
