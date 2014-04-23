# rebootlockd

rebootlockd is a reboot manager for the CoreOS update engine which uses
etcd to ensure that only a subset of a cluster of machines is rebooting
at any given time.

## Usage

### Listing the Holders

```
$ rebootlockctl status
Available: 0
Max: 1

MACHINE
69d27b356a94476da859461d3a3bc6fd
```

### Unlock Holders

In some cases a machine may go away permanently or semi-permanently while
holding a reboot lock. A system administrator can clear this lock using the
unlock command.

```
$ rebootlockctl unlock 69d27b356a94476da859461d3a3bc6fd
```

### Maximum Sempahore

By default the reboot lock only allows a single holder. However, a user may
want more than a single machine to be upgrading at a time. This can be done by
increasing the semaphore count.

```
$ rebootlockctl set-max 4
Old: 1
New: 4
```

## Keyspace

### Semaphore

Key: `coreos.com/updateengine/rebootlock/semaphore`

The semaphore is a json document that describes a simple semaphore that clients
swap to take the lock. When it is first created it will be initialized like so:

```json
{
	"semaphore": 1,
	"max": 1,
	"holders": []
}
```

To take the lock a client the document will be swaped with this:

```json
{
	"semaphore": 0,
	"max": 1,
	"holders": [
		"69d27b356a94476da859461d3a3bc6fd"
	]
}
```

### Holders Directory

Key: `coreos.com/updateengine/rebootlock/holders/<machineID>`


When a rebootlock client takes the lock it should write information about
itself to the holders directory. This should be a JSON document with the
following information:

```json
{
	"machineID": "69d27b356a94476da859461d3a3bc6fd",
	"startTime": 1397496396
}
```

This information is used to show an admin who is holding the semaphore and to
resolve problems. The holder must delete themselves with a swap operation
before incrementing the semaphore. This lets an administrator safely clear a
reboot lock for a client that experienced a power outage or other failure
before being able to release the lock.
