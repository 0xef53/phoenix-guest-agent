package devconn

import (
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/netutil"
)

type DeviceConn struct {
	devpath string
	f       *os.File
	Closed  bool
}

func DialDevice(devpath string) (*DeviceConn, error) {
	f, err := os.OpenFile(devpath, syscall.O_RDWR|syscall.O_ASYNC|syscall.O_NDELAY, 0666)
	if err != nil {
		return nil, err
	}

	return &DeviceConn{f: f, devpath: devpath}, nil
}

func (c *DeviceConn) Read(b []byte) (n int, err error) {
	return c.f.Read(b)
}

func (c *DeviceConn) Write(b []byte) (n int, err error) {
	return c.f.Write(b)
}

func (c *DeviceConn) Close() error {
	// There is no need to close the serial port every time.
	// So just do nothing.
	return nil
}

func (c *DeviceConn) closeDevice() error {
	c.Closed = true

	return c.f.Close()
}

func (c *DeviceConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "virtio-port:" + c.devpath, Net: "virtio"}
}

func (c *DeviceConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "qemu-host", Net: "virtio"}
}

func (c *DeviceConn) SetDeadline(t time.Time) error {
	return c.f.SetDeadline(t)
}

func (c *DeviceConn) SetReadDeadline(t time.Time) error {
	return c.f.SetReadDeadline(t)
}

func (c *DeviceConn) SetWriteDeadline(t time.Time) error {
	return c.f.SetWriteDeadline(t)
}

type DeviceListener struct {
	mu     sync.Mutex
	conn   *DeviceConn
	closed bool
}

func ListenDevice(devpath string) (net.Listener, error) {
	c, err := DialDevice(devpath)
	if err != nil {
		laddr := &net.UnixAddr{Name: "virtio-port:" + devpath, Net: "virtio"}
		return nil, &net.OpError{Op: "dial", Net: "virtio", Source: laddr, Addr: nil, Err: err}
	}

	return netutil.LimitListener(&DeviceListener{conn: c}, 1), nil
}

func (ln *DeviceListener) ok() bool {
	return ln != nil && ln.conn != nil && !ln.closed
}

func (ln *DeviceListener) Accept() (net.Conn, error) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	// Virtio causes us to spin here when no process is attached
	// to host-side chardev.
	// Seep a bit to mitigate this.
	time.Sleep(time.Second)

	if !ln.ok() {
		return nil, syscall.EINVAL
	}

	return ln.conn, nil
}

func (ln *DeviceListener) Close() error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if !ln.ok() {
		return syscall.EINVAL
	}

	if ln.closed {
		return nil
	}
	ln.closed = true

	return ln.conn.closeDevice()
}

func (ln *DeviceListener) Addr() net.Addr {
	if ln.ok() {
		return ln.conn.LocalAddr()
	}

	return nil
}

func (ln *DeviceListener) CloseConn() error {
	if !ln.ok() {
		return syscall.EINVAL
	}

	return ln.conn.Close()
}
