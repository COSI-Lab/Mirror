package main

import (
	"sort"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

var formatter = opts.FuncOpts("function (params) { return `<span style=\"width: 1em; height: 1em; background-color: ${params.color}; border-radius: 50%; display: inline-block; vertical-align: middle;\"></span> ${params.name}: ${humanFileSize(params.value, true)}`; }")

func getPieChart() *charts.Pie {
	// Query the database
	fields, err := QueryBytesSentByProject()

	if err != nil {
		logging.Warn("getPieChart", err)
	}

	// Convert query to pie data
	var rawdata []opts.PieData

	var total float64 = 0
	for key, value := range fields {
		project, ok := projects[key]
		var style *opts.ItemStyle
		if ok {
			style = &opts.ItemStyle{
				Color: project.Color,
			}
		}

		rawdata = append(rawdata, opts.PieData{
			Name:      key,
			Value:     float64(value),
			ItemStyle: style,
		})

		total += float64(value)
	}

	// Sort data by value
	sort.Slice(rawdata, func(i, j int) bool {
		return rawdata[i].Value.(float64) < rawdata[j].Value.(float64)
	})

	// combine smaller values
	var data []opts.PieData
	other := &opts.PieData{
		Name:  "other",
		Value: float64(0),
		ItemStyle: &opts.ItemStyle{
			Color: "#777777",
		},
	}

	for _, value := range rawdata {
		if value.Value.(float64) < 0.01*total {
			other.Value = other.Value.(float64) + value.Value.(float64)
		} else {
			value.Value = value.Value.(float64)
			data = append(data, value)
		}
	}

	other.Value = other.Value.(float64)

	// sort data again
	sort.Slice(data, func(i, j int) bool {
		return data[i].Value.(float64) > data[j].Value.(float64)
	})

	data = append(data, *other)

	// Create a new pie chart
	pie := charts.NewPie()
	pie.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Project usage (last 24 hours)",
		}),
		charts.WithTooltipOpts(
			opts.Tooltip{
				Trigger:   "item",
				Show:      true,
				Formatter: formatter,
			},
		),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Orient: "vertical",
			Left:   "right",
		}),
	)

	pie.AddSeries("pie", data)

	pie.JSAssets = types.OrderedSet{}
	pie.JSAssets.Init("https://go-echarts.github.io/go-echarts-assets/assets/echarts.min.js")

	return pie
}
