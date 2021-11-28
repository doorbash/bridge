package constant

import "net"

type UdpConn struct {
	net.PacketConn
	Addr *net.UDPAddr
}

func (u UdpConn) Read(b []byte) (int, error) {
	n, _, err := u.ReadFrom(b)
	return n, err
}

func (u UdpConn) Write(b []byte) (int, error) {
	return u.WriteTo(b, u.RemoteAddr())
}

func (u UdpConn) RemoteAddr() net.Addr {
	return u.Addr
}
