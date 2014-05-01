package updateengine

import (
	"fmt"
	"github.com/coreos/locksmith/third_party/github.com/godbus/dbus"
	"os"
	"strconv"
)

const (
	dbusPath			= "/com/coreos/update1"
	dbusInterface			= "com.coreos.update1.Manager"
	dbusMember			= "StatusUpdate"
	dbusMemberInterface		= dbusInterface + "." + dbusMember
	signalBuffer			= 32	// TODO(bp): What is a reasonable value here?
	UpdateStatusUpdatedNeedReboot	= "UPDATE_STATUS_UPDATED_NEED_REBOOT"
)

type Client struct {
	conn	*dbus.Conn
	object	*dbus.Object
	ch	chan *dbus.Signal
}

func New() (c *Client, err error) {
	c = new(Client)

	c.conn, err = dbus.SystemBusPrivate()
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Getuid()))}
	err = c.conn.Auth(methods)
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	err = c.conn.Hello()
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	c.object = c.conn.Object("com.coreos.update1", dbus.ObjectPath(dbusPath))

	// Setup the filter for the StatusUpdate signals
	match := fmt.Sprintf("type='signal',interface='%s',member='%s'", dbusInterface, dbusMember)

	call := c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, match)
	if call.Err != nil {
		return nil, err
	}

	c.ch = make(chan *dbus.Signal, signalBuffer)
	c.conn.Signal(c.ch)

	return c, nil
}

func (c *Client) RebootNeededSignal(rcvr chan Status) {
	for {
		signal := <-c.ch
		s := NewStatus(signal.Body)
		println(s.String())
		if s.CurrentOperation == UpdateStatusUpdatedNeedReboot {
			rcvr <- s
		}
	}
}

func (c *Client) GetStatus() (result Status, err error) {
	call := c.object.Call(dbusInterface+".GetStatus", 0)
	err = call.Err
	if err != nil {
		return
	}

	result = NewStatus(call.Body)

	return
}

func (c *Client) Close() {
	c.conn.Close()
	close(c.ch)
}
