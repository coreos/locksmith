package updateengine

import (
	"fmt"
	"github.com/godbus/dbus"
	"os"
	"strconv"
)

const (
	dbusInterface = "org.chromium.UpdateEngineInterface"
	dbusPath = "/org/chromium/UpdateEngineInterface"
	dbusMember = "StatusUpdate"
	dbusMemberInterface = dbusInterface + "." + dbusMember
	signalBuffer = 32 // TODO(bp): What is a reasonable value here?
	UpdateStatusUpdatedNeedReboot = "UPDATE_STATUS_UPDATED_NEED_REBOOT"
)

type Client struct {
	conn *dbus.Conn
	object *dbus.Object
	ch chan *dbus.Signal
}

func New() (c *Client, err error) {
	c = new(Client)

	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Getuid()))}
	err = conn.Auth(methods)
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	err = conn.Hello()
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	// Setup the filter for the StatusUpdate signals
	match := fmt.Sprintf("type='signal',interface='%s',member='%s'", dbusInterface, dbusMember)
	call := c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, match)
	if call.Err != nil {
		return nil, err
	}

	c.conn = conn
	c.ch = make(chan *dbus.Signal, signalBuffer)
	c.conn.Signal(c.ch)

	return c, nil
}

func (c *Client) RebootNeededSignal(rcvr chan bool) {
	for {
		signal := <-c.ch
		switch signal.Name {
		case dbusMemberInterface:
			current_operation := signal.Body[2].(string)
			if current_operation == UpdateStatusUpdatedNeedReboot {
				rcvr <- true
			}
		}
	}
}

func (c *Client) Close() {
	c.conn.Close()
	close(c.ch)
}
