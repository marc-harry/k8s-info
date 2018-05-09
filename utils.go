package main

import (
	"fmt"
	"os"
	"time"

	inf "gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

func asString(res interface{}) string {
	return fmt.Sprintf("%s", res)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getPercentage(first *resource.Quantity, second *resource.Quantity) *inf.Dec {
	val := new(inf.Dec).QuoRound(first.AsDec(), second.AsDec(), 2, inf.RoundCeil)
	per := new(inf.Dec).Mul(val, inf.NewDec(100, 0))
	return per
}

const (
	maxDuration time.Duration = 1<<63 - 1
)

func getTimeSince(value time.Time) string {
	upTimeUnit := "d"
	upTime := time.Since(value)
	if upTime == maxDuration {
		return ""
	}
	upTimeValue := upTime.Hours() / float64(24)
	if upTimeValue >= 1 {
		upTimeUnit = "d"
		return fmt.Sprintf("%d%s", int(upTimeValue), upTimeUnit)
	}
	upTimeValue = upTime.Hours()
	if upTimeValue >= 1 {
		upTimeValue = upTime.Hours()
		upTimeUnit = "h"
		return fmt.Sprintf("%d%s", int(upTimeValue), upTimeUnit)
	}
	upTimeValue = upTime.Minutes()
	if upTimeValue >= 1 {
		upTimeValue = upTime.Minutes()
		upTimeUnit = "m"
		return fmt.Sprintf("%d%s", int(upTimeValue), upTimeUnit)
	}

	upTimeValue = upTime.Seconds()
	upTimeUnit = "s"
	return fmt.Sprintf("%d%s", int(upTimeValue), upTimeUnit)
}
