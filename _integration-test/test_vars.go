package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type (
	MachineDriver struct {
		name string
	}
)

var (
	machineBinary      = "machine"
	machineTestDrivers []MachineDriver
)

func init() {
	// allow filtering driver tests
	if machineTests := os.Getenv("MACHINE_TESTS"); machineTests != "" {
		tests := strings.Split(machineTests, " ")
		for _, test := range tests {
			mcn := MachineDriver{
				name: test,
			}
			machineTestDrivers = append(machineTestDrivers, mcn)
		}
	} else {
		machineTestDrivers = []MachineDriver{
			{
				name: "virtualbox",
			},
			{
				name: "digitalocean",
			},
		}
	}

	// find machine binary
	if machineBin := os.Getenv("MACHINE_BINARY"); machineBin != "" {
		machineBinary = machineBin
	} else {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Printf("ERROR: unable to get current directory: %s", err)
			os.Exit(1)
		}
		machineBinary = filepath.Join(wd, "../machine")
	}
	if _, err := os.Stat(machineBinary); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("ERROR: unable to find the machine binary.  Have you tried building it?")
		} else {
			fmt.Printf("ERROR: %s", err)
		}
		os.Exit(1)
	}
}
