# Mirror

WIP monolithic software for [mirror](https://mirror.clarkson.edu) that handles
- [x] Defining what projects we host from a centralized config
- [ ] Parsing passwords from config
- [x] Recording nginx bandwidth per repo
- [ ] Recording rsync bandwidth
- [ ] Recording tranmission bittorrent bandwidth
- [ ] Recording total network bandwidth
- [ ] Exposing nginx bandwidth per repo
- [ ] Exposing rsync bandwidth
- [ ] Exposing tranmission bittorrent bandwidth
- [ ] Exposing total network bandwidth
- [x] Mirror map of real time downloads
- [ ] Periodically syncing projects
- [ ] Exposing sync status
- [x] Discord webhook integration
- [ ] Notifies our discord server when things fail 

## Frontend


## Want live rebuilding?

```
go get github.com/codegangsta/gin
gin -p 3000 -b mirror
```

Enjoy!

## Env File Formatting
```
HOOK_URL = url
INFLUX_TOKEN = token
```

## GeoLite2 Attribution

This software includes GeoLite2 data created by MaxMind, available from [www.maxmind.com](https://www.maxmind.com)