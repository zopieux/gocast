package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/grandcat/zeroconf"
	"github.com/zopieux/gocast/constants"
)

// GenerateStableUUID generates a 32-character hex UUID (without dashes) derived from machine identity.
func GenerateStableUUID(salt ...string) string {
	var idBase string
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagLoopback) == 0 && (iface.Flags&net.FlagUp) != 0 && len(iface.HardwareAddr) > 0 {
				idBase = iface.HardwareAddr.String()
				break
			}
		}
	}
	if idBase == "" {
		h, _ := os.Hostname()
		idBase = h
	}
	s := "gocast"
	if len(salt) > 0 && salt[0] != "" {
		s = salt[0]
	}
	hash := sha256.Sum256([]byte(idBase + s))
	return strings.ToLower(hex.EncodeToString(hash[:16]))
}

// StartAdvertising registers the _googlecast._tcp service on mDNS.
func StartAdvertising(uuidStr, friendlyName, model string, port int) (*zeroconf.Server, error) {
	txtRecords := []string{
		fmt.Sprintf("id=%s", uuidStr),
		fmt.Sprintf("fn=%s", friendlyName),
		fmt.Sprintf("md=%s", model),
		"ve=05",
		"ca=4101",
		"st=0",
		"nf=1",
		"rs=",
	}

	server, err := zeroconf.Register(
		uuidStr,
		constants.MDNSService,
		constants.MDNSDomain,
		port,
		txtRecords,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register zeroconf service: %w", err)
	}

	slog.Info("mDNS Cast service registered", "uuid", uuidStr, "name", friendlyName, "model", model, "port", port)
	return server, nil
}
