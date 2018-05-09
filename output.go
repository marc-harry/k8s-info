package main

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	typesv1 "k8s.io/api/core/v1"
)

func outputData(headers []string, data [][]string) {
	fmt.Printf("Kubernetes Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}

func outputFailing(dataMap map[string]typesv1.PodPhase) {
	data := [][]string{}
	for podName, podInfo := range dataMap {
		data = append(data, []string{podName, asString(podInfo)})
	}
	fmt.Printf("Failing Pod Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pod", "Status"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}
