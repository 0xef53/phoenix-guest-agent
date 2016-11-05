General
-------

- All requests to the Agent should be terminated with CRLF
- Request may contain a string field "tag" to uniquely identify a response from Agent
- For more details, see the QMP-specification

Commands
--------

### ping

**Returns:** the version of the guest agent

**Example:**

    -> { "execute": "ping" }
    <- { "return": "0.4" }


### get-commands

**Returns:** list of available commands

**Example:**

    -> { "execute": "get-commands" }
    <- { "return": ["agent-shutdown", "ping", "get-netifaces"] }


### agent-shutdown

Shutdown the guest agent. If the agent started by the init process, it will be automatically launched. Therefore in this case the command reloads agent.

**Returns:** true on success

**Example:**

    -> { "execute": "agent-shutdown" }
    <- { "return": true }


### sysinfo

**Returns:** summary information about the guest system: uptime, load average, utsname, logged in users, ram/swap usage, block devices stat, etc.

**Example:**

    -> { "execute": "sysinfo" }
    <- {
         "long_bit": 64,
         "disks": [
           {
             "size_used": 621900,
             "name": "/dev/vda",
             "size_total": 5131008,
             "size_avail": 4230592,
             "is_mounted": true,
             "mountpoint": "/"
           },
           {
             "size_used": 42468,
             "name": "/dev/vdb",
             "size_total": 42468,
             "size_avail": 0,
             "is_mounted": true,
             "mountpoint": "/lib/modules"
           }
         ],
         "users": [
           {
             "device": "pts/0",
             "host": "79.172.60.6",
             "name": "root",
             "login_time": 1478346362
           }
         ],
         "ram": {
           "buffer": 14928,
           "total": 1032644,
           "free": 838220
         },
         "uptime": 12488,
         "uname": {
           "sysname": "Linux",
           "domain": "(none)",
           "nodename": "vm-61b27f65",
           "machine": "x86_64",
           "version": "#66-Ubuntu SMP Wed Oct 19 14:12:37 UTC 2016",
           "release": "4.4.0-45-generic"
         },
         "loadavg": {
           "5m": 0.0087890625,
           "15m": 0,
           "1m": 0.0185546875
         },
         "swap": {
           "total": 0,
           "free": 0
         }
       }


### file-open

Open a file in the guest system.

**Arguments:**

- `path` -- full path to the file in the guest system
- `mode` -- "r" or "w". Default is "r"
- `perm` -- an access mode in dec of the file. Default is 420 (644 in oct)

**Returns:** a file descriptor

**Example:**

    -> { "execute": "file-open", arguments: { "path": "/path/to/file" } }
    <- { "return": 9 }


### file-read

Read from the open file descriptor in the guest system. If EOF, the file will be automatically closed.

**Arguments:** 

- `handle_id` -- a file descriptor

**Returns:**

- `bufb64` -- a base64-encoded string
- `eof` -- true, if EOF

**Example:**

    -> { "execute": "file-read", "arguments": { "handle_id": 9 } }
    <- { "return": { "bufb64": "Ny4yCg==", "eof": false } }


### file-write

Write to the open file descriptor in the guest system. If EOF, the file will be automatically closed.

**Arguments:** 

- `handle_id` -- a file descriptor
- `bufb64` -- a base64-encoded string representing data to be written
- `eof` -- true, if this is the last chunk of data

**Returns:** true on success

**Example:**

    -> { "execute": "file-write", "arguments": { "bufb64": "Ny4wCg==", "handle_id": 9, "eof": false } }
    <- { "return": true }


### file-close

Close an open file descriptor in the guest system.

**Arguments:** 

- `handle_id` -- a file descriptor

**Returns:** true on success

**Example:**

    -> { "execute": "file-close", "arguments': { "handle_id": 9 } }
    <- { "return": true }


### get-file-md5sum

**Arguments:** 

- `path` -- full path to the file in the guest system

**Returns:** a md5 hash of the file

**Example:**

    -> { "execute": "get-file-md5sum", "arguments": { "path": "/path/to/file" } }
    <- { "return": "a81dbdee5126e79a7099b890c5a36ebe" }


### file-chown

Change the file uid and gid in the guest system.

**Arguments:** 

- `path` -- full path to the file in the guest system
- `uid` -- a numeric uid of the file. Default is 0
- `gid` -- a numeric gid of the file. Default is 0

**Returns:** true on success

**Example:**

    -> { "execute": "file-chown", "arguments": { "path": "/path/to/file", "uid": 0, "gid": 0 } }
    <- { "return": true }


### file-chmod

Change the file mode bits in the guest system.

**Arguments:** 

- `path` -- full path to the file in the guest system
- `perm` -- an access mode in dec of the file which to be set

**Returns:** true on success

**Example:**

    -> { "execute": "file-chmod", "arguments": { "path": "/path/to/file", "perm": 493 } }
    <- { "return": true }

### file-stat

**Arguments:** 

- `path` -- full path to the file in the guest system

**Returns:** a stat structure of the file

**Example:**

    -> { "execute": "file-stat", "arguments": { "path": "/path/to/file" } }
    <- { "return": { "name": "file", "isdir": false, "stat": {<stat_structure>} } }


### directory-create

**Arguments:** 

- `path` -- full path to the new directory in the guest system
- `perm` -- an access mode in dec of the directory which to be set. Default is 493 (755 in oct)

**Returns:** true on success

**Example:**

    -> { "execute": "directory-create", "arguments": { "path": "/path/to/file" } }
    <- { "return": true }


### directory-list

**Arguments:** 

- `path` -- full path to the directory in the guest system
- `n` -- the number of elements in the returned list (if <= 0, then returns all files in the directory). Default is 0.

**Returns:** a list of the file stat structures in this directory

**Example:**

    -> { "execute": "directory-list", "arguments": { "path": "/path/to/directory" } }
    <- { "return": [<file_stat_structures>] }


### fs-freeze

Sync and freeze all freezable local guest file systems.

Since version 0.6 this command ignores file systems located on loop and dm devices. It's a necessary measure to prevent blockages due to multiple mounts of the same devices (e.g., bind mounts).

**Returns:** true on success

**Example:**

    -> { "execute": "fs-freeze" }
    <- { "return": true }


### fs-unfreeze

Unfreeze all frozen guest file systems.

**Returns:** true on success

**Example:**

    -> { "execute": "fs-unfreeze" }
    <- { "return": true }


### get-freeze-status

**Returns:** true if file systems are frozen or false otherwise

**Example:**

    -> { "execute": "get-freeze-status" }
    <- { "return": false }


### get-netifaces

**Returns:** list of network parameters: IP-adresses, MAC-adresses etc.

**Example:**

    -> { "execute": "get-netifaces" }
    <- { "return": [
           { "hwaddr": "",
             "index": 1,
             "flags": "up|loopback",
             "name": "lo", 
             "ips": ["127.0.0.1/8", "::1/128"]
           },
           { "hwaddr": "00:16:3e:02:92:07",
             "index": 2,
             "flags": "up|broadcast|multicast",
             "name": "eth0",
             "ips': ["193.107.236.127/32", "fe80::216:3eff:fe02:9207/64"']
           } ]
       }


### ipaddr-add

Add the IP-address to the network interface in the guest system.

The old name `linux-ipaddr-add` is also available, but is deprecated.

**Arguments:** 

- `ip` -- an IP-address in CIDR format
- `ifname` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "linux-ipaddr-add", "arguments": { "ip": "192.168.55.77/32", "ifname": "eth0" } }
    <- { "return": true }


### ipaddr-del

Remove the IP-address from the network interface in the guest system.

The old name `linux-ipaddr-del` is also available, but is deprecated.

**Arguments:** 

- `ip` -- an IP-address in CIDR format
- `ifname` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "linux-ipaddr-del", "arguments": { "ip": "192.168.55.77/32", "ifname": "eth0" } }
    <- { "return": true }


### net-iface-up

Bring up the specified network interface in the guest system.

**Arguments:** 

- `ifname` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "net-iface-up", "arguments": { "ifname": "eth1" } }
    <- { "return": true }


### net-iface-down

Bring down the specified network interface in the guest system.

**Arguments:** 

- `ifname` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "net-iface-down", "arguments": { "ifname": "eth1" } }
    <- { "return": true }


### get-route-list

**Arguments:** 

- `family` -- an integer representation of family type from linux/socket.h: AF_UNSPEC, AF_INET or AF_INET6. Default is AF_UNSPEC

**Returns:** a list of routing table entries

**Example:**

    -> { "execute": "get-route-list", "arguments": { "family": 2 } }
    <- { "return": [
           { "ifname": "eth0",
             "scope": 0,
             "dst": { "ip": "","mask": "" },
             "src": "",
             "gateway": "10.11.11.11"
           },
           { "ifname": "eth0",
             "scope": 253,
             "dst": { "ip": "10.11.11.11", "mask": "255.255.255.255" },
             "src": "",
             "gateway": ""
           } ]
       }

This output is equivalent to:

    $ ip route show
    default via 10.11.11.11 dev eth0
    10.11.11.11 dev eth0  scope link


### route-add

Add a new entry to the routing table in the guest system.

**Arguments:** 

- `ifname` -- a name of output interface
- `dst` -- a destination prefix of the route
- `src` -- a source address to prefer when sending to the destinations covered by the route prefix
- `gateway` -- an address of the nexthop router

**Returns:** true on success

**Example:**

    -> { "execute": "route-add", "arguments": { "ifname": "eth1", "dst": "8.8.8.8/32", "src": "", "gateway": "10.0.0.254" } }
    <- { "return": true }

This command is identical to `ip route add 8.8.8.8/32 via 10.0.0.254`


### route-del

Remove an entry from the routing table in the guest system.

**Arguments:** 

- `ifname` -- a name of output interface
- `dst` -- a destination prefix of the route
- `src` -- a source address to prefer when sending to the destinations covered by the route prefix
- `gateway` -- an address of the nexthop router

**Returns:** true on success

**Example:**

    -> { "execute": "route-del", "arguments": { "ifname": "eth0", "dst": "172.16.1.0/22", "src": "", "gateway": "10.0.0.254" } }
    <- { "return": true }

This command is identical to `ip route del 172.16.1.0/22 via 10.0.0.254`
