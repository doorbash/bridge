package outbound

import (
	"context"
	"errors"

	"fmt"
	"net"
	"strconv"

	"github.com/doorbash/bridge/common/structure"
	obfs "github.com/doorbash/bridge/component/simple-obfs"
	"github.com/doorbash/bridge/component/socks5"
	v2rayObfs "github.com/doorbash/bridge/component/v2ray-plugin"
	C "github.com/doorbash/bridge/constant"

	"github.com/doorbash/go-shadowsocks2/core"
)

type ShadowSocks struct {
	*Base
	cipher core.Cipher

	// obfs
	obfsMode    string
	obfsOption  *simpleObfsOption
	v2rayOption *v2rayObfs.Option
}

type ShadowSocksOption struct {
	Name       string                 `proxy:"name"`
	Server     string                 `proxy:"server"`
	Port       int                    `proxy:"port"`
	Password   string                 `proxy:"password"`
	Cipher     string                 `proxy:"cipher"`
	UDP        bool                   `proxy:"udp,omitempty"`
	Plugin     string                 `proxy:"plugin,omitempty"`
	PluginOpts map[string]interface{} `proxy:"plugin-opts,omitempty"`
}

type simpleObfsOption struct {
	Mode string `obfs:"mode,omitempty"`
	Host string `obfs:"host,omitempty"`
}

type v2rayObfsOption struct {
	Mode           string            `obfs:"mode"`
	Host           string            `obfs:"host,omitempty"`
	Path           string            `obfs:"path,omitempty"`
	TLS            bool              `obfs:"tls,omitempty"`
	Headers        map[string]string `obfs:"headers,omitempty"`
	SkipCertVerify bool              `obfs:"skip-cert-verify,omitempty"`
	Mux            bool              `obfs:"mux,omitempty"`
}

func (ss *ShadowSocks) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	switch ss.obfsMode {
	case "tls":
		c = obfs.NewTLSObfs(c, ss.obfsOption.Host)
	case "http":
		_, port, _ := net.SplitHostPort(ss.addr)
		c = obfs.NewHTTPObfs(c, ss.obfsOption.Host, port)
	case "websocket":
		var err error
		c, err = v2rayObfs.NewV2rayObfs(c, ss.v2rayOption)
		if err != nil {
			return nil, fmt.Errorf("%s connect error: %w", ss.addr, err)
		}
	}
	c = ss.cipher.StreamConn(c)
	_, err := c.Write(serializesSocksAddr(metadata))
	return c, err
}

func (ss *ShadowSocks) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	con, err := ss.dialer.DialContext(ctx, ss.addrMetadata)
	if err != nil {
		return nil, err
	}
	c, err := ss.StreamConn(con, metadata)
	return NewConn(c, ss), err
}

func (ss *ShadowSocks) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pk, err := ss.dialer.DialUDP(ss.addrMetadata)
	if err != nil {
		return nil, err
	}
	addr, err := resolveUDPAddr("udp", ss.addr)
	if err != nil {
		return nil, err
	}
	pc := ss.cipher.PacketConn(pk)
	return newPacketConn(&ssPacketConn{PacketConn: pc, rAddr: addr}, ss), nil
}

func NewShadowSocks(option ShadowSocksOption) (*ShadowSocks, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))
	cipher := option.Cipher
	password := option.Password
	ciph, err := core.PickCipher(cipher, nil, password)
	if err != nil {
		return nil, fmt.Errorf("ss %s initialize error: %w", addr, err)
	}

	var v2rayOption *v2rayObfs.Option
	var obfsOption *simpleObfsOption
	obfsMode := ""

	decoder := structure.NewDecoder(structure.Option{TagName: "obfs", WeaklyTypedInput: true})
	if option.Plugin == "obfs" {
		opts := simpleObfsOption{Host: "bing.com"}
		if err := decoder.Decode(option.PluginOpts, &opts); err != nil {
			return nil, fmt.Errorf("ss %s initialize obfs error: %w", addr, err)
		}

		if opts.Mode != "tls" && opts.Mode != "http" {
			return nil, fmt.Errorf("ss %s obfs mode error: %s", addr, opts.Mode)
		}
		obfsMode = opts.Mode
		obfsOption = &opts
	} else if option.Plugin == "v2ray-plugin" {
		opts := v2rayObfsOption{Host: "bing.com", Mux: true}
		if err := decoder.Decode(option.PluginOpts, &opts); err != nil {
			return nil, fmt.Errorf("ss %s initialize v2ray-plugin error: %w", addr, err)
		}

		if opts.Mode != "websocket" {
			return nil, fmt.Errorf("ss %s obfs mode error: %s", addr, opts.Mode)
		}
		obfsMode = opts.Mode
		v2rayOption = &v2rayObfs.Option{
			Host:    opts.Host,
			Path:    opts.Path,
			Headers: opts.Headers,
			Mux:     opts.Mux,
		}

		if opts.TLS {
			v2rayOption.TLS = true
			v2rayOption.SkipCertVerify = opts.SkipCertVerify
			v2rayOption.SessionCache = getClientSessionCache()
		}
	}

	ss := &ShadowSocks{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.Shadowsocks,
			udp:  option.UDP,
		},
		cipher: ciph,

		obfsMode:    obfsMode,
		v2rayOption: v2rayOption,
		obfsOption:  obfsOption,
	}

	ss.addrMetadata, err = C.NewMetadata(addr)

	return ss, err
}

type ssPacketConn struct {
	net.PacketConn
	rAddr net.Addr
}

func (spc *ssPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	packet, err := socks5.EncodeUDPPacket(socks5.ParseAddrToSocksAddr(addr), b)
	if err != nil {
		return
	}
	return spc.PacketConn.WriteTo(packet[3:], spc.rAddr)
}

func (spc *ssPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, _, e := spc.PacketConn.ReadFrom(b)
	if e != nil {
		return 0, nil, e
	}

	addr := socks5.SplitAddr(b[:n])
	if addr == nil {
		return 0, nil, errors.New("parse addr error")
	}

	udpAddr := addr.UDPAddr()
	if udpAddr == nil {
		return 0, nil, errors.New("parse addr error")
	}

	copy(b, b[len(addr):])
	return n - len(addr), udpAddr, e
}
