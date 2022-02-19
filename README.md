# Mirror

WIP monolithic software for [mirror](https://mirror.clarkson.edu) that handles
- [x] Defining what projects we host from a centralized config
- [x] Parsing passwords from config
- [ ] Generating rsyncd.conf from config
- [ ] Manage torrents using config
- [ ] Config reloading using SIGHUP
- [x] Recording nginx bandwidth per repo
- [ ] Recording rsync bandwidth
- [ ] Recording rsyncd bandwidth
- [ ] Recording tranmission bittorrent bandwidth
- [ ] Recording total network bandwidth
- [ ] Exposing nginx bandwidth per repo
- [ ] Exposing rsync bandwidth
- [ ] Exposing tranmission bittorrent bandwidth
- [ ] Exposing total network bandwidth
- [x] Mirror map of real time downloads
- [x] Mirror map generated from project config
- [x] Periodically syncing projects
- [ ] Exposing sync status per project
- [x] Discord webhook integration
- [x] Notifies our discord server when things fail

## Frontend

- [ ] Highlight nav links on hover
- [ ] "Welcome to Mirror" on home page
- [ ] Mobile friendly navbar
- [ ] Table of contents on distro and software pages
- [ ] "Designed By: COSI", mirror contact "mirroradmin@clarkson.edu"
- [ ] "Full screen" map mode
- [ ] Somehow make the map look nice on mobile devices
- [ ] Move the "longer mirror history" off of Meeting Minutes
- [ ] Need new content about reporting errors and requesting new projects through github issues
- [ ] please use a nicer font
- [ ] software page should mostly mirror the distribuions page
- [ ] On the stats page please put "construction tux" :)
- [ ] Make better use of the header `h1, h2, h3, ...` tags

## Development

First you need to install the latest version of [golang](https://golang.org/doc/install). Then make sure `~/go/bin` is in your `$PATH`. Now you build and run the project using [gin](https://github.com/codegangsta/gin).

```
go install github.com/codegangsta/gin@latest
gin --all -p 3002 -b Mirror -i
```

## Env File Formatting
```
# Discord Webhook URL
HOOK_URL=url
PING_ID=id

# InfluxDB RW Token
INFLUX_TOKEN=token

# File to tail NGINX access logs, if empty then we read the static ./access.log file
NGINX_TAIL=/var/log/nginx/access.log

# Maxmind DB token
MAXMIND_LICENSE_KEY=key

# Set to _anything_ to completely disable rsync
RSYNC_DISABLE=1

# Set to _anything_ to add the --dry-run flag to the rsync jobs
RSYNC_DRY_RUN=1

# Directory to store the rsync log files, if empty then we don't keep logs
# RSYNC_LOGS will be created if it doesn't exist
RSYNC_LOGS=/var/log/mirror/rsync
```

## GeoLite2 Attribution

This software includes GeoLite2 data created by MaxMind, available from [www.maxmind.com](https://www.maxmind.com)