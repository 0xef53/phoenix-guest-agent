General
-------

- All requests to the Agent should be terminating with CRLF
- Request may contain a string field "tag" to uniquely identify a response from Agent
- For more details, see the QMP-specification

Commands
--------

### ping

**Returns:** the version of the guest agent

**Example:**

    -> { "execute": "ping" }
    <- { "return": "0.1" }


### get-commands

**Returns:** list of available commands

**Example:**

    -> { "execute": "get-commands" }
    <- { "return": ["agent-shutdown", "ping", "get-netifaces"] }


### agent-shutdown

Shutdown the guest agent. If the agent started by the init process, it will be automatically launched. Therefore in this case the command reload agent.

**Returns:** true on success

**Example:**

    -> { "execute": "agent-shutdown" }
    <- { "return": true }


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


### linux-ipaddr-add

Add the IP-address to the network interface in the guest system.

**Arguments:** 

- `ip` -- an IP-address in CIDR format
- `dev` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "linux-ipaddr-add", "arguments": { "ip": "192.168.55.77/32", "dev": "eth0" } }
    <- { "return": true }


### linux-ipaddr-del

Remove the IP-address from the network interface in the guest system.

**Arguments:** 

- `ip` -- an IP-address in CIDR format
- `dev` -- a network interface name

**Returns:** true on success

**Example:**

    -> { "execute": "linux-ipaddr-del", "arguments": { "ip": "192.168.55.77/32", "dev": "eth0" } }
    <- { "return": true }

