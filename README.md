# Mirror

WIP monolithic software for [mirror](https://mirror.clarkson.edu) that handles
- [ ] Total bandwidth statistics
- [ ] Bandwidth statistics per repo
- [ ] Mirror map of real time downloads
- [ ] Defining what projects we host from a centralized config
- [ ] Periodically syncing projects
- [ ] Exposing sync status and other data from an API
- [ ] Backing statistics up to a database
- [ ] Notifies our discord server when things fail

## GeoLite2 Attribution

This software includes GeoLite2 data created by MaxMind, available from [www.maxmind.com](https://www.maxmind.com)

Env File Formatting
- HOOK_URL = url
- INFLUX_TOKEN = token