//go:build !go1.21

package main

type any = interface{}

func slicesContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
