package updateengine

import (
	"fmt"
	"github.com/godbus/dbus"
	"os"
	"strconv"
)

const (
	dbusInterface = "com.coreos.update1.Engine"
	dbusPath = "/com/coreos/update1/Engine"
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

	c.object = c.conn.Object("com.coreos.update1.Engine",
		dbus.ObjectPath("/com/coreos/update1/Engine"))

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

func (c *Client) GetStatus() error {
	result := make([][]interface{}, 0)
	err := c.object.Call("com.coreos.update1.Engine.GetStatus", 0).Store(&result)
	if err != nil {
		return err
	}
	println(result)

	return nil
}

func (c *Client) Close() {
	c.conn.Close()
	close(c.ch)
}
