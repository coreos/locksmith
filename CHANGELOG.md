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
