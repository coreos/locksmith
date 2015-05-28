# locksmith

locksmith is a reboot manager for the CoreOS update engine which uses
etcd to ensure that only a subset of a cluster of machines are rebooting
at any given time. `locksmithd` runs as a daemon on CoreOS machines and is
responsible for controlling the reboot behaviour after updates.

## Configuration

There are three different strategies that `locksmithd` can use after the update
engine has successfully applied an update:

- `etcd-lock` - reboot after first taking a lock in etcd.
- `reboot` - reboot immediately without taking a lock.
- `best-effort` - if etcd is running, then do `etcd-lock`; otherwise, `reboot`.
- `off` - causes locksmithd to exit and do nothing.

These strategies can be configured via `/etc/coreos/update.conf` with a line that looks like:

```
REBOOT_STRATEGY=reboot
```

The reboot strategy can also be configured through [cloud-config](https://github.com/coreos/coreos-cloudinit/blob/master/Documentation/cloud-config.md#update).

The default strategy is `best-effort`.

## Usage

`locksmithctl` is a simple client that can be use to introspect and control the
lock used by locksmith.  It is installed by default on CoreOS.

Run `locksmithctl -help` for a list of command-line options.

All command-line options can also be specified using environment variables with
a `LOCKSMITHCTL_` prefix. For example, the `-endpoint` argument can be set
using `LOCKSMITHCTL_ENDPOINT`.

### Connecting to multiple endpoints

Multiple endpoints can be specified by passing the `-endpoint=<url>` option for
each endpoint, or by passing a comma-separated list of endpoints, e.g.:

    -endpoint=<url>,<url>

Specifying multiple endpoints using an environment variable is supported by
passing a comma-delimited list, e.g.:

    LOCKSMITHCTL_ENDPOINT=<url>,<url>

### Listing the Holders

```
$ locksmithctl status
Available: 0
Max: 1

MACHINE ID
69d27b356a94476da859461d3a3bc6fd
```

### Unlock Holders

In some cases a machine may go away permanently or semi-permanently while
holding a reboot lock. A system administrator can clear the lock of a specific
machine using the unlock command:

```
$ locksmithctl unlock 69d27b356a94476da859461d3a3bc6fd
```

### Maximum Semaphore

By default the reboot lock only allows a single holder. However, a user may
want more than a single machine to be upgrading at a time. This can be done by
increasing the semaphore count.

```
$ locksmithctl set-max 4
Old: 1
New: 4
```

## Groups

`locksmithd` coordinates the reboot lock in groups of machines. The default
group is "", or the empty string. `locksmithd` will only coordinate the reboot
lock with other machines in the same group.

The purpose of groups is to allow faster updating of certain sets of machines
while maintaining availability of certain services. For example, in a cluster
of 5 CoreOS machines with all machines in the default group, if you have 2 load
balancers and run `locksmithctl set-max 2`, then it is possible that both load
balancers would be rebooted at the same time, interrupting the service they
provide. However, if the load balancers are put into their own group named "lb",
and both the default group and the "lb" group have a max holder of 1, two
reboots can occur at once, but both load balancers will never reboot at the same
time.

### Configuring groups

To place machines in a group other than the default, `locksmithd` must be started
with the `-group=groupname` flag or set the `LOCKSMITHD_GROUP=groupname` environment
variable.

To control the semaphore of a group other than the default, you must invoke
`locksmithctl` with the `-group=groupname` flag or set the `LOCKSMITHCTL_GROUP=groupname`
environment variable.

## Reboot windows

`locksmithd` can be configured to only reboot during certain timeframes. The
reboot window is configured through two environment variables,
`REBOOT_WINDOW_START` and `REBOOT_WINDOW_LENGTH`. Here is an example configuration:

```
REBOOT_WINDOW_START=14:00
REBOOT_WINDOW_LENGTH=1h
```

This would configure `locksmithd` to only reboot between 2pm and 3pm. Optionally,
a day of week may be specified for the start of the window:

```
REBOOT_WINDOW_START=Thu 23:00
REBOOT_WINDOW_LENGTH=1h30m
```

This would configure `locksmithd` to only reboot the system on Thursday after 11pm,
or on Friday before 12:30am.

Currently, the only supported values for the day of week are short day names,
e.g. `Sun`, `Mon`, `Tue`, `Wed`, `Thu`, `Fri`, and `Sat`, but the day of week can
be upper or lower case. The time of day must be specified in 24-hour time format.
The window length is expressed as input to go's [time.ParseDuration][time.ParseDuration]
function.

[time.ParseDuration]: http://godoc.org/time#ParseDuration

## Implementation details 

The following section describes how locksmith works under the hood.

### Semaphore

locksmith uses a [semaphore][semaphore] in etcd, located at the key
`coreos.com/updateengine/rebootlock/semaphore`, to coordinate the reboot lock.
If a non-default group name is used, the etcd key will be
`coreos.com/updateengine/rebootlock/groups/$groupname/semaphore`.

The semaphore is a JSON document, describing a simple semaphore, that clients [swap][cas]
to take the lock. 

When it is first created it will be initialized like so:

```json
{
	"semaphore": 1,
	"max": 1,
	"holders": []
}
```

For a client to take the lock, the document is swapped with this:

```json
{
	"semaphore": 0,
	"max": 1,
	"holders": [
		"69d27b356a94476da859461d3a3bc6fd"
	]
}
```

[semaphore]: http://en.wikipedia.org/wiki/Semaphore_(programming)
[cas]: https://github.com/coreos/etcd/blob/master/Documentation/api.md#atomic-compare-and-swap
