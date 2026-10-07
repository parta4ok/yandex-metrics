package entities

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
		RandomValue:
		return true
	default:
		return false
	}
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
