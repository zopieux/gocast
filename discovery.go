package gocast

import (
	"github.com/grandcat/zeroconf"
	"github.com/zopieux/gocast/discovery"
)

// GenerateStableUUID generates a stable hex UUID derived from machine identity.
func GenerateStableUUID(salt ...string) string {
	return discovery.GenerateStableUUID(salt...)
}

// StartAdvertising registers the Google Cast receiver service on mDNS.
func StartAdvertising(uuidStr, friendlyName, model string, port int) (*zeroconf.Server, error) {
	return discovery.StartAdvertising(uuidStr, friendlyName, model, port)
}
