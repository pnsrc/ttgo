//go:build windows

package client

import "gvisor.dev/gvisor/pkg/tcpip"

const (
	tunDeviceName = "TrustTunnel"
	tunHeaderLen  = 0
)

func tunWritePacket(pkt []byte, proto tcpip.NetworkProtocolNumber) []byte {
	return pkt
}
