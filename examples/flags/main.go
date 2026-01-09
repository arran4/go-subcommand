package main

import "time"

// MyCommand is a subcommand `root flags`
// param name (-n --name) Your name (default: "World")
// param age (-a) Your age (default: 18)
// param verbose (-v) Verbose mode
// param timeout (-t) Timeout duration (default: 10s)
func MyCommand(name string, age int, verbose bool, timeout time.Duration) {}
