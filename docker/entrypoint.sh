#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}
UMASK=${UMASK:-002}

# Apply umask for shared homelab volume writes (002 ensures group-writable rw-rw-r-- / rwxrwxr-x)
umask "$UMASK"

# If running as root, manage user/group IDs and drop privileges via su-exec
if [ "$(id -u)" = '0' ]; then
    # Configure shelfd group
    if ! getent group shelfd >/dev/null 2>&1; then
        groupadd -g "$PGID" shelfd 2>/dev/null || addgroup -g "$PGID" shelfd 2>/dev/null || true
    else
        groupmod -o -g "$PGID" shelfd 2>/dev/null || true
    fi

    # Configure shelfd user
    if ! getent passwd shelfd >/dev/null 2>&1; then
        useradd -u "$PUID" -g "$PGID" -d /data -s /bin/sh shelfd 2>/dev/null || \
        adduser -u "$PUID" -G shelfd -h /data -s /bin/sh -D shelfd 2>/dev/null || true
    else
        usermod -o -u "$PUID" -g "$PGID" shelfd 2>/dev/null || true
    fi

    # Ensure internal data directories exist and are owned by shelfd
    mkdir -p /data/covers
    chown -R "$PUID:$PGID" /data

    # Audiobookshelf Sanctity (@librarian):
    # NEVER recursively chown /library! In multi-gigabyte or terabyte shared homelab setups,
    # recursive chowning causes long startup delays and unwanted inode touches.
    if [ -d "/library" ] && [ ! -w "/library" ]; then
        echo "[shelfd] Warning: /library is not writable by UID $PUID"
    fi

    exec su-exec "$PUID:$PGID" "$@"
fi

# Already non-root
exec "$@"
