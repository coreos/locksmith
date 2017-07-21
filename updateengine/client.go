// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updateengine

import (
	"fmt"
	"os"
	"strconv"

	"github.com/godbus/dbus"
)

const (
	dbusPath            = "/com/coreos/update1"
	dbusInterface       = "com.coreos.update1.Manager"
	dbusMember          = "StatusUpdate"
	dbusMemberInterface = dbusInterface + "." + dbusMember
	signalBuffer        = 32 // TODO(bp): What is a reasonable value here?
	// UpdateStatusUpdatedNeedReboot is the status returned by updateengine on
	// dbus when a reboot is needed
	UpdateStatusUpdatedNeedReboot = "UPDATE_STATUS_UPDATED_NEED_REBOOT"
)

// Client is a dbus client subscribed to updateengine status updates
type Client struct {
	conn   *dbus.Conn
	object *dbus.Object
	ch     chan *dbus.Signal
}

// New returns a Client connected to dbus over a private connection with a
// subscription to updateengine.
func New() (c *Client, err error) {
	c = new(Client)

	c.conn, err = dbus.SystemBusPrivate()
	if err != nil {
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

// RebootNeededSignal watches the updateengine status update subscription and
// passes the status on the rcvr channel whenever the status update is
// UpdateStatusUpdatedNeedReboot. The stop channel terminates the function.
// This function is intended to be called as a goroutine
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

// GetStatus returns the current status of updateengine
// it returns an error if there is a problem getting the status from dbus
func (c *Client) GetStatus() (result Status, err error) {
	call := c.object.Call(dbusInterface+".GetStatus", 0)
	err = call.Err
	if err != nil {
		return
	}

	result = NewStatus(call.Body)

	return
}
