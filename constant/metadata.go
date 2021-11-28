package constant

import (
	"encoding/json"
	"net"
	"strconv"
)

// Socks addr type
const (
	ATypIPv4       = 1
	ATypDomainName = 3
	ATypIPv6       = 4

	TCP NetWork = iota
	UDP
)

type NetWork int

func (n NetWork) String() string {
	if n == TCP {
		return "tcp"
	}
	return "udp"
}

func (n NetWork) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

// Metadata is used to store connection address
type Metadata struct {
	NetWork  NetWork `json:"network"`
	DstIP    net.IP  `json:"destinationIP"`
	DstPort  string  `json:"destinationPort"`
	AddrType int     `json:"-"`
	Host     string  `json:"host"`
}

func (m *Metadata) RemoteAddress() string {
	return net.JoinHostPort(m.String(), m.DstPort)
}

func (m *Metadata) SourceAddress() string {
	return "nil"
}

func (m *Metadata) Resolved() bool {
	return m.DstIP != nil
}

func (m *Metadata) UDPAddr() *net.UDPAddr {
	if m.NetWork != UDP || m.DstIP == nil {
		return nil
	}
	port, _ := strconv.Atoi(m.DstPort)
	return &net.UDPAddr{
		IP:   m.DstIP,
		Port: port,
	}
}

func (m *Metadata) String() string {
	if m.Host != "" {
		return m.Host
	} else if m.DstIP != nil {
		return m.DstIP.String()
	} else {
		return "<nil>"
	}
}

func (m *Metadata) Valid() bool {
	return m.Host != "" || m.DstIP != nil
}

func NewMetadata(addr string) (*Metadata, error) {
	md := &Metadata{}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		md.Host = host
	} else {
		ipv4 := ip.To4()
		if ipv4 != nil {
			md.DstIP = ipv4
			md.AddrType = ATypIPv4
		} else {
			md.DstIP = ip
			md.AddrType = ATypIPv6
		}
	}
	md.DstPort = port
	return md, nil
}
