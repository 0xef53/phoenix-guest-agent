#!/bin/bash
set -eu

for FNAME in $@ ; do
    if [[ ! "$FNAME" =~ \.pb\.go$ ]] ; then
        continue
    fi

    sed -i "$FNAME" \
        -e 's/Mtu/MTU/g' \
        -e 's/Uid/UID/g' \
        -e 's/Gid/GID/g' \
        -e 's/WithoutSsh/WithoutSSH/g' \
        -e 's/WithoutTcp/WithoutTCP/g'

done

exit 0
