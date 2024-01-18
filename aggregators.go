package main

import (
	"net"
	"time"

	"github.com/COSI-Lab/Mirror/aggregator"
	"github.com/COSI-Lab/Mirror/config"
	"github.com/COSI-Lab/Mirror/logging"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

func StartNGINXAggregator(reader api.QueryAPI, writer api.WriteAPI, config *config.File) (chan<- aggregator.NGINXLogEntry, time.Time, error) {
	nginxAg := aggregator.NewNGINXProjectAggregator()
	nginxAg.AddMeasurement("nginx", func(re aggregator.NGINXLogEntry) bool {
		return true
	})

	// Add subnet aggregators
	for name, subnetStrings := range config.Subnets {
		subnets := make([]*net.IPNet, 0)
		for _, subnetString := range subnetStrings {
			_, subnet, err := net.ParseCIDR(subnetString)
			if err != nil {
				logging.Warnf("Failed to parse subnet %q for %q", subnetString, name)
				continue
			}
			subnets = append(subnets, subnet)
		}

		if len(subnets) == 0 {
			logging.Warn("No valid subnets for", name)
			continue
		}

		nginxAg.AddMeasurement(name, func(re aggregator.NGINXLogEntry) bool {
			for _, subnet := range subnets {
				if subnet.Contains(re.IP) {
					return true
				}
			}
			return false
		})

		logging.Infof("Added subnet aggregator for %q", name)
	}

	nginxMetrics := make(chan aggregator.NGINXLogEntry)
	nginxLastUpdated, err := aggregator.StartAggregator[aggregator.NGINXLogEntry](reader, writer, nginxAg, nginxMetrics)

	return nginxMetrics, nginxLastUpdated, err
}

func StartRSYNCAggregator(reader api.QueryAPI, writer api.WriteAPI) (chan<- aggregator.RSCYNDLogEntry, time.Time, error) {
	rsyncAg := aggregator.NewRSYNCProjectAggregator()

	rsyncMetrics := make(chan aggregator.RSCYNDLogEntry)
	rsyncLastUpdated, err := aggregator.StartAggregator[aggregator.RSCYNDLogEntry](reader, writer, rsyncAg, rsyncMetrics)

	return rsyncMetrics, rsyncLastUpdated, err
}
