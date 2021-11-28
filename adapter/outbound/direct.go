package outbound

import (
	"context"
	"log"

	"fmt"
	"net"

	"github.com/doorbash/bridge/component/dialer"
	C "github.com/doorbash/bridge/constant"
)

type Direct struct {
	*Base
}

func (d *Direct) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	return c, nil
}

func (d *Direct) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	log.Printf("Direct: dialing %s ....\n", metadata.RemoteAddress())
	c, err := dialer.DialContext(ctx, "tcp", metadata.RemoteAddress())
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", d.addr, err)
	}
	tcpKeepAlive(c)
	return NewConn(c, d), err
}

func (d *Direct) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := dialer.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	addr, err := resolveUDPAddr("udp", metadata.RemoteAddress())
	if err != nil {
		return nil, err
	}

	return newPacketConn(&dPacketConn{PacketConn: pc, rAddr: addr}, d), nil
}

func NewDirect() (*Direct, error) {
	return &Direct{
		Base: &Base{
			tp: C.Shadowsocks,
		},
	}, nil
}

type dPacketConn struct {
	net.PacketConn
	rAddr net.Addr
}
