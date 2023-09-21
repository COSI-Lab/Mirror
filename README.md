# Mirror

Monolithic software for our [mirror](https://mirror.clarkson.edu) that handles the website, tracking, and scheduling systems. We use an influxdb time series database for storage.

![preview](./preview.png)

## Setup

```cli
git clone --recurse-submodule https://github.com/COSI-Lab/Mirror
```

## `.env`

Secrets and some configuration is managed through creating a `.env` file.

```text
# Maxmind DB token to update the database. Omit and we'll only use a local copy if it exists
# Note: The maxmind DB license requires we use an up-to-date copy
MAXMIND_LICENSE_KEY=

# InfluxDB Token
INFLUX_TOKEN=

# "true" if we only read from the database
INFLUX_READ_ONLY=

# Location on disk to save torrents to leave
# empty to disable the torrent syncing system
TORRENT_DIR=

# File to tail NGINX access logs, if empty then we read the static ./access.log file
NGINX_TAIL=/var/log/nginx/access.log

# File to tail rsyncd log file. If empty then we read a local ./rsyncd.log file
RSYNCD_TAIL=/var/log/rsyncd.log

# Set to "true" to pause scheduling sync tasks
SCHEDULER_PAUSED=true

# "true" if the --dry-run flag to the rsync jobs
# and we skip other scripts pulls
SYNC_DRY_RUN=true

# Directory to store the rsync log files, if empty then we don't keep logs. It will be created if it doesn't exist.
RSYNC_LOGS=

# Secret pull token
PULL_TOKEN=token
```

## Dependencies

Quick-Fedora-Mirror requires `zsh`

raspbian-tools requires `python3` and it's recommended to pip install the `urllib3` module

## Hardware

```text
8x Black Diamond M-1333TER-8192BD23 8GB DDR3 RAM

Samsung EVO 870 1TB SATA SSD

8x 16 TB IronWolf Pro NAS Drives - ST16000NE000

HP 671798-001 10gb Ethernet Network Interface Card NIC Board

Some random pcie riser for PCIE 3 M.2 ssd cache

Sabrent SB-RKT4P-1TB
```

## GeoLite2 Attribution

This software includes GeoLite2 data created by MaxMind, available from [www.maxmind.com](https://www.maxmind.com)
