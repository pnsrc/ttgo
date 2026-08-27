//go:build !darwin && !windows

package client

import "gvisor.dev/gvisor/pkg/tcpip"

const (
	tunDeviceName = "tun0"
	tunHeaderLen  = 0
)

func tunWritePacket(pkt []byte, proto tcpip.NetworkProtocolNumber) []byte {
	return pkt
}
