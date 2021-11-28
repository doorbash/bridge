package outbound

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/doorbash/bridge/constant"
	C "github.com/doorbash/bridge/constant"
)

type Base struct {
	name         string
	addr         string
	addrMetadata *C.Metadata
	tp           C.AdapterType
	udp          bool
	dialer       C.ProxyAdapter
}

func (b *Base) Name() string {
	return b.name
}

func (b *Base) Type() C.AdapterType {
	return b.tp
}

func (b *Base) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	return c, errors.New("no support")
}

func (b *Base) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("no support")
}

func (b *Base) SupportUDP() bool {
	return b.udp
}

func (b *Base) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": b.Type().String(),
	})
}

func (b *Base) Addr() string {
	return b.addr
}

func (b *Base) SetDialer(dialer C.ProxyAdapter) {
	b.dialer = dialer
}

func NewBase(name string, addr string, tp C.AdapterType, udp bool, dialer C.Proxy) *Base {
	return &Base{name, addr, nil, tp, udp, dialer}
}

type conn struct {
	net.Conn
	chain C.Chain
}

func (c *conn) Chains() C.Chain {
	return c.chain
}

func (c *conn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

func NewConn(c net.Conn, a C.ProxyAdapter) C.Conn {
	return &conn{c, []string{a.Name()}}
}

type packetConn struct {
	net.PacketConn
	chain C.Chain
}

func (c *packetConn) Chains() C.Chain {
	return c.chain
}

func (c *packetConn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

func newPacketConn(pc net.PacketConn, a C.ProxyAdapter) C.PacketConn {
	return &packetConn{pc, []string{a.Name()}}
}

type Proxy struct {
	C.ProxyAdapter
}

func (p *Proxy) Dial(metadata *C.Metadata) (C.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	return p.DialContext(ctx, metadata)
}

func (p *Proxy) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	return p.ProxyAdapter.DialContext(ctx, metadata)
}

// URLTest get the delay for the specified URL, t ms
func (p *Proxy) URLTest(ctx context.Context, URL string, resolveAddr bool) (string, uint16, error) {
	metadata, err := urlToMetadata(URL)
	if err != nil {
		return "", 0, err
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			md := &C.Metadata{
				AddrType: C.ATypIPv4,
				NetWork:  C.UDP,
				DstIP:    net.IPv4(8, 8, 8, 8),
				DstPort:  "53",
			}

			pk, err := p.DialUDP(md)

			if err != nil {
				log.Println(err)
				return nil, err
			}

			addr := &net.UDPAddr{
				IP:   md.DstIP,
				Port: 53,
			}

			return constant.UdpConn{
				pk,
				addr,
			}, nil
		},
	}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if resolveAddr && !metadata.Resolved() {
			log.Println("resolving", metadata.Host)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ips, err := resolver.LookupIP(ctx, "ip", metadata.Host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no address associated with this domain %s", metadata.Host)
			}
			metadata.DstIP = ips[0]
			metadata.Host = ""
			if len(metadata.DstIP) == net.IPv4len {
				metadata.AddrType = C.ATypIPv4
			} else {
				metadata.AddrType = C.ATypIPv6
			}
		}
		c, err := p.Dial(&metadata)
		if err != nil {
			return nil, err
		}
		return c, nil
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: dialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: tcpTimeout,
	}

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return "", 0, err
	}

	req = req.WithContext(ctx)
	sTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}

	defer resp.Body.Close()
	t := uint16(time.Since(sTime).Milliseconds())

	r, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", 0, err
	}

	return string(r), t, nil
}

func NewProxy(adapter C.ProxyAdapter) *Proxy {
	return &Proxy{adapter}
}
