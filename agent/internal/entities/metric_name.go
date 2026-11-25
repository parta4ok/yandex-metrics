package entities

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const cpuUtilizationPrefix = "CPUutilization"

type MName string

const (
	Alloc         MName = "Alloc"
	BuckHashSys   MName = "BuckHashSys"
	Frees         MName = "Frees"
	GCCPUFraction MName = "GCCPUFraction"
	GCSys         MName = "GCSys"
	HeapAlloc     MName = "HeapAlloc"
	HeapIdle      MName = "HeapIdle"
	HeapInuse     MName = "HeapInuse"
	HeapObjects   MName = "HeapObjects"
	HeapReleased  MName = "HeapReleased"
	HeapSys       MName = "HeapSys"
	LastGC        MName = "LastGC"
	Lookups       MName = "Lookups"
	MCacheInuse   MName = "MCacheInuse"
	MCacheSys     MName = "MCacheSys"
	MSpanInuse    MName = "MSpanInuse"
	MSpanSys      MName = "MSpanSys"
	Mallocs       MName = "Mallocs"
	NextGC        MName = "NextGC"
	NumForcedGC   MName = "NumForcedGC"
	NumGC         MName = "NumGC"
	OtherSys      MName = "OtherSys"
	PauseTotalNs  MName = "PauseTotalNs"
	StackInuse    MName = "StackInuse"
	StackSys      MName = "StackSys"
	Sys           MName = "Sys"
	TotalAlloc    MName = "TotalAlloc"
	PollCount     MName = "PollCount"
	RandomValue   MName = "RandomValue"
	TotalMemory   MName = "TotalMemory"
	FreeMemory    MName = "FreeMemory"
)

func (m MName) IsValid() bool {
	switch m {
	case Alloc,
		BuckHashSys,
		Frees,
		GCCPUFraction,
		GCSys,
		HeapAlloc,
		HeapIdle,
		HeapInuse,
		HeapObjects,
		HeapReleased,
		HeapSys,
		LastGC,
		Lookups,
		MCacheInuse,
		MCacheSys,
		MSpanInuse,
		MSpanSys,
		Mallocs,
		NextGC,
		NumForcedGC,
		NumGC,
		OtherSys,
		PauseTotalNs,
		StackInuse,
		StackSys,
		Sys,
		TotalAlloc,
		PollCount,
		RandomValue,
		TotalMemory,
		FreeMemory:
		return true
	default:
		return m.isCPUUtilization()
	}
}

func NewCPUUtilization(index int) (MName, error) {
	if index <= 0 {
		return "", errors.Wrap(ErrInvalidParam, "new cpu utilization. index is not positive")
	}

	return MName(cpuUtilizationPrefix + strconv.Itoa(index)), nil
}

func (m MName) isCPUUtilization() bool {
	index, found := strings.CutPrefix(string(m), cpuUtilizationPrefix)
	if !found {
		return false
	}

	value, err := strconv.Atoi(index)

	return err == nil && value > 0 && strconv.Itoa(value) == index
}

func (m MName) MType() MType {
	if m == PollCount {
		return Counter
	}

	if m.IsValid() {
		return Gauge
	}

	return ""
}
