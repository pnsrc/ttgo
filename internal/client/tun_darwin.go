//go:build darwin

package client

import (
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
)

const (
	tunDeviceName = "utun"
	tunHeaderLen  = 4
)

func tunWritePacket(pkt []byte, proto tcpip.NetworkProtocolNumber) []byte {
	buf := make([]byte, tunHeaderLen, tunHeaderLen+len(pkt))
	switch proto {
	case ipv4.ProtocolNumber:
		buf[3] = 2 // AF_INET
	case ipv6.ProtocolNumber:
		buf[3] = 30 // AF_INET6
	}
	return append(buf, pkt...)
}
