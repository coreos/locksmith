### v0.3.4

The environment variables controlling reboot windows (`REBOOT_WINDOW_START`,
`REBOOT_WINDOW_LENGTH`) have been renamed to include the prefix `LOCKSMITHD_`
to maintain consistency with other locksmithd environment variables.

The old environment variables are still read to maintain compatibility with
locksmithd v0.3.1 to v0.3.3.

### v0.3.3

Remove dependency on update-engine.service from locksmithd.service. If
update-engine failed to start, systemd wouldn't start locksmith and the restart
logic only applies if the service can be started.

### v0.3.2

Set GOMAXPROCS=1 in the locksmithd systemd service to keep behavior consistent between builds using Go 1.5 and previous versions.

### v0.3.1

v0.3.1 is the first release with a changelog :-)

There are also a number of new features in this release, including [groups](README.md#groups), an [`off` strategy](README.md#configuration), and [reboot windows](README.md#reboot-windows).

Full list of changes since v0.3.0:
- New features
  - "groups" feature, facilitating partitioned co-ordinating of reboots (#70)
  - "off" strategy, which will cause locksmith to perform no action and shut itself down (#79)
  - reboot windows, allowing control over when reboots occur (#80)
- Bug fixes
  - daemon now considers strategy when attempting to unlock, rather than just blindly checking the local etcd's activeness (#86)
  - updateengine client no longer attempts to close a dbus connection if the connecion failed (#83)
- Other changes
  - greater verbosity of error messages in the case of unlocking failures (#82)
