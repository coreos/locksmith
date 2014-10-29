/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package updateengine

import (
	"fmt"
	"os"
	"strconv"

	"github.com/coreos/locksmith/Godeps/_workspace/src/github.com/godbus/dbus"
)

const (
	dbusPath                      = "/com/coreos/update1"
	dbusInterface                 = "com.coreos.update1.Manager"
	dbusMember                    = "StatusUpdate"
	dbusMemberInterface           = dbusInterface + "." + dbusMember
	signalBuffer                  = 32 // TODO(bp): What is a reasonable value here?
	UpdateStatusUpdatedNeedReboot = "UPDATE_STATUS_UPDATED_NEED_REBOOT"
)

type Client struct {
	conn   *dbus.Conn
	object *dbus.Object
	ch     chan *dbus.Signal
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

func (c *Client) RebootNeededSignal(rcvr chan Status, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case signal := <-c.ch:
			s := NewStatus(signal.Body)
			println(s.String())
			if s.CurrentOperation == UpdateStatusUpdatedNeedReboot {
				rcvr <- s
			}
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
