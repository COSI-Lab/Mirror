# Mirror

Monolithic software for our [mirror](https://mirror.clarkson.edu) that handles the website, tracking, and scheduling systems. We use an influxdb time series database for storage.

![preview](./preview.png)

## Setup

```cli
git clone --recurse-submodule https://github.com/COSI-Lab/Mirror
```

## `.env`

Secrets are managed through creating a `.env` file.

```text
# Maxmind DB token to update the database. Omit and we'll only use a local copy if it exists
# Note: The maxmind DB license requires we use an up-to-date copy
MAXMIND_LICENSE_KEY=

# InfluxDB Token must support read/write access to the database
INFLUX_TOKEN=
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
