# Mirror

Monolithic software for our [mirror](https://mirror.clarkson.edu) that handles the website, tracking, and scheduling systems. We use an influxdb time series database for storage. It is important to clone this repo with the `--recurse-submodules` flag!

## TODO LIST

- [ ] The Mirror "book" documentation
- [ ] Manage torrents using config
- [ ] Recording rsync bandwidth
- [ ] Recording tranmission bittorrent bandwidth
- [ ] Make the map look nice on mobile devices

Statistics Page:

- [ ] Exposing nginx bandwidth per repo (pie chart)
- [ ] Exposing rsync bandwidth
- [ ] Exposing tranmission bittorrent bandwidth
- [ ] Exposing total network bandwidth

## Env File Formatting

```text
# Discord Webhook URL and id to ping when things panic
# Omit either and the bot will not communicate with discord
HOOK_URL=url
PING_ID=id

# Maxmind DB token to update the database, omit and we'll only use a local copy if it exists
MAXMIND_LICENSE_KEY=key

# InfluxDB RW Token
INFLUX_TOKEN=token

# "true" if we only read from the database (still uses a rw token)
INFLUX_READ_ONLY=true

# File to tail NGINX access logs, if empty then we read the static ./access.log file
NGINX_TAIL=/var/log/nginx/access.log

# File to tail rsyncd log file. If empty then we read a local ./rsyncd.log file
RSYNCD_TAIL=/var/log/rsyncd.log

# "true" if the --dry-run flag to the rsync jobs
# and we skip other scripts pulls
SYNC_DRY_RUN=true

# Directory to store the rsync log files, if empty then we don't keep logs. It will be created if it doesn't exist.
RSYNC_LOGS=/tmp/mirror/

# If we should cache the result of executing templates
WEB_SERVER_CACHE=true

# Secret push token
PULL_TOKEN=token
```

## Dependencies

Quick-Fedora-Mirror requires zsh

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
