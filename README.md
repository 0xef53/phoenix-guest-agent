Phoenix guest agent
-------------------
[![Build Status](https://drone.io/github.com/0xef53/phoenix-guest-agent/status.png)](https://drone.io/github.com/0xef53/phoenix-guest-agent/latest)

Phoenix is a guest-side agent for qemu-kvm virtual machines. It provides communication with the
guest system using virtio-serial port, and allows to perform some commands in the guest system from
the master.


### Supported functions

- working with guest's files and directories: reading/writing files, setting mode/uid/gid, creating directories, listing directories etc.
- querying and setting network parameters: adding/removing IP-adresses, getting summary information.
- freezing/thawing guest filesystems.


### How to use

Launch a qemu-kvm process with additional options for the character device driver required to
communicate with the guest agent:

    qemu-system-x86_64 \
        -chardev socket,id=ga0,path=/var/run/guestagent.sock,server,nowait \
        -device virtio-serial-pci \
        -device virtserialport,chardev=ga0,name=org.guest-agent.0

On the guest system, launch the guest agent like this:

    phoenix-ga -p /dev/virtio-ports/org.guest-agent.0

Now, we can talk to guest agent from the master server:

    master# socat - UNIX-CONNECT:/var/run/guestagent.sock
    { "execute": "get-commands", "tag": "abc" }
    { "return": ["get-commands", "agent-shutdown", "ping", "get-netifaces", "linux-ipaddr-add", "linux-ipaddr-del", "file-open", "file-close", "file-read", "file-write", "get-file-md5sum"], "tag": "abc" }

    { "execute": "ping", "tag": "def" }
    { "return": "0.1", "tag": "def" }

Communication with the guest agent occurs at QMP-like protocol. The success response contains
the field "return" with the results of command execution. The error response contains the field
"error" with a base64-encoded description. For details see the [commands documentation](docs/commands.md).

Since version 0.4 the field "error" also contains an extended code status, which can take next values:

- an unsigned number (`errno`) describing an error condition
- and `-1` in case if an extended code is not defined

E.g:

    { "error": { "bufb64": "anVzdCBhIHRlc3QgZXJyb3I=", "code": -1 }, "tag": "8be" }


### Resource consumption

Measurements were performed using the cpuacct cgroups controller and pmap.

In the idling agent consumed about 17 seconds of CPU time per day and about 2.5 Mb RSS.

### Getting binary

Latest version is available [here](https://drone.io/github.com/0xef53/phoenix-guest-agent/files/phoenix-ga)

### Installing from source

    mkdir phoenix-ga && cd phoenix-ga
    export GOPATH=$(pwd); go get -v -tags netgo -ldflags '-s -w' github.com/0xef53/phoenix-guest-agent/...
    mv bin/phoenix-guest-agent bin/phoenix-ga

### Supported OS

GNU/Linux

### License

MIT
