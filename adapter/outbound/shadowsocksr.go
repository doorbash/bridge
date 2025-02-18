package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/doorbash/bridge/component/ssr/obfs"
	"github.com/doorbash/bridge/component/ssr/protocol"
	C "github.com/doorbash/bridge/constant"
	"github.com/doorbash/go-shadowsocks2/core"
	"github.com/doorbash/go-shadowsocks2/shadowstream"
)

type ShadowSocksR struct {
	*Base
	cipher   *core.StreamCipher
	obfs     obfs.Obfs
	protocol protocol.Protocol
}

type ShadowSocksROption struct {
	Name          string `proxy:"name"`
	Server        string `proxy:"server"`
	Port          int    `proxy:"port"`
	Password      string `proxy:"password"`
	Cipher        string `proxy:"cipher"`
	Obfs          string `proxy:"obfs"`
	ObfsParam     string `proxy:"obfs-param,omitempty"`
	Protocol      string `proxy:"protocol"`
	ProtocolParam string `proxy:"protocol-param,omitempty"`
	UDP           bool   `proxy:"udp,omitempty"`
}

func (ssr *ShadowSocksR) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c = obfs.NewConn(c, ssr.obfs)
	c = ssr.cipher.StreamConn(c)
	conn, ok := c.(*shadowstream.Conn)
	if !ok {
		return nil, fmt.Errorf("invalid connection type")
	}
	iv, err := conn.ObtainWriteIV()
	if err != nil {
		return nil, err
	}
	c = protocol.NewConn(c, ssr.protocol, iv)
	_, err = c.Write(serializesSocksAddr(metadata))
	return c, err
}

func (ssr *ShadowSocksR) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	// log.Printf("ShadowSocksR: DialContext %s ...\n", metadata.RemoteAddress())
	con, err := ssr.dialer.DialContext(ctx, ssr.addrMetadata)
	if err != nil {
		return nil, err
	}
	c, err := ssr.StreamConn(con, metadata)
	return NewConn(c, ssr), err
}

func (ssr *ShadowSocksR) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	// log.Printf("ShadowSocksR: DialUDP %s ...\n", metadata.RemoteAddress())
	pk, err := ssr.dialer.DialUDP(ssr.addrMetadata)
	if err != nil {
		return nil, err
	}
	addr, err := resolveUDPAddr("udp", ssr.addr)
	if err != nil {
		return nil, err
	}
	pc := ssr.cipher.PacketConn(pk)
	pc = protocol.NewPacketConn(pc, ssr.protocol)
	return newPacketConn(&ssPacketConn{PacketConn: pc, rAddr: addr}, ssr), nil
}

func NewShadowSocksR(option ShadowSocksROption) (*ShadowSocksR, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	coreCiph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize cipher error: %w", addr, err)
	}
	ciph, ok := coreCiph.(*core.StreamCipher)
	if !ok {
		return nil, fmt.Errorf("%s is not a supported stream cipher in ssr", cipher)
	}

	obfs, err := obfs.PickObfs(option.Obfs, &obfs.Base{
		IVSize:  ciph.IVSize(),
		Key:     ciph.Key,
		HeadLen: 30,
		Host:    option.Server,
		Port:    option.Port,
		Param:   option.ObfsParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize obfs error: %w", addr, err)
	}

	protocol, err := protocol.PickProtocol(option.Protocol, &protocol.Base{
		IV:     nil,
		Key:    ciph.Key,
		TCPMss: 1460,
		Param:  option.ProtocolParam,
	})
	if err != nil {
		return nil, fmt.Errorf("ssr %s initialize protocol error: %w", addr, err)
	}
	protocol.SetOverhead(obfs.GetObfsOverhead() + protocol.GetProtocolOverhead())

	ssr := &ShadowSocksR{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.ShadowsocksR,
			udp:  option.UDP,
		},
		cipher:   ciph,
		obfs:     obfs,
		protocol: protocol,
	}

	ssr.addrMetadata, err = C.NewMetadata(addr)

	return ssr, err
}
