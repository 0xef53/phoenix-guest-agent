Phoenix guest agent
-------------------
[![Build Status](https://drone.io/github.com/0xef53/phoenix-guest-agent/status.png)](https://drone.io/github.com/0xef53/phoenix-guest-agent/latest)

Phoenix is a guest-side agent for qemu-kvm virtual machines. It provides communication with the
guest system using virtio-vsock interface (AF_VSOCK) or virtio-serial port, and allows to perform some commands
in the guest system from the host.


### Supported functions

- working with guest's files and directories: reading/writing files, setting mode/uid/gid, creating directories, listing directories etc.
- querying and setting network parameters: adding/removing IP-adresses, getting summary information.
- freezing/thawing guest filesystems.
- querying summary information about the guest: uptime, load average, utsname, logged in users, ram/swap usage, block devices stat, etc.


### How to use

...

