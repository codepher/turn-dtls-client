package main

import (
	"net"
	"time"

	"github.com/pion/dtls/v2"
)

type UdpPacketConn struct {
	conn *dtls.Conn
}

func NewUdpDtls(d *dtls.Conn) *UdpPacketConn {
	return &UdpPacketConn{d}
}

func (d *UdpPacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	n, err = d.conn.Read(b)
	return n, d.conn.RemoteAddr(), err
}

func (d *UdpPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	return d.conn.Write(b)
}

func (d *UdpPacketConn) Close() error {
	return d.conn.Close()
}
func (d *UdpPacketConn) LocalAddr() net.Addr {
	return d.conn.LocalAddr()
}
func (d *UdpPacketConn) SetDeadline(t time.Time) error {
	return d.conn.SetDeadline(t)
}

func (d *UdpPacketConn) SetReadDeadline(t time.Time) error {
	return d.conn.SetReadDeadline(t)
}
func (d *UdpPacketConn) SetWriteDeadline(t time.Time) error {
	return d.conn.SetWriteDeadline(t)
}

//// ReadFrom方法从连接读取一个数据包，并将有效信息写入b
//// ReadFrom方法可能会在超过某个固定时间限制后超时返回错误，该错误的Timeout()方法返回真
//// 返回写入的字节数和该数据包的来源地址
//ReadFrom(b []byte) (n int, addr Addr, err error)
//// WriteTo方法将有效数据b写入一个数据包发送给addr
//// WriteTo方法可能会在超过某个固定时间限制后超时返回错误，该错误的Timeout()方法返回真
//// 在面向数据包的连接中，写入超时非常罕见
//WriteTo(b []byte, addr Addr) (n int, err error)
//// Close方法关闭该连接
//// 会导致任何阻塞中的ReadFrom或WriteTo方法不再阻塞并返回错误
//Close() error
//// 返回本地网络地址
//LocalAddr() Addr
//// 设定该连接的读写deadline
//SetDeadline(t time.Time) error
//// 设定该连接的读操作deadline，参数t为零值表示不设置期限
//// 如果时间到达deadline，读操作就会直接因超时失败返回而不会阻塞
//SetReadDeadline(t time.Time) error
//// 设定该连接的写操作deadline，参数t为零值表示不设置期限
//// 如果时间到达deadline，写操作就会直接因超时失败返回而不会阻塞
//// 即使写入超时，返回值n也可能>0，说明成功写入了部分数据
//SetWriteDeadline(t time.Time) error
