package entities

type MType string

const (
	Counter MType = "counter"
	Gauge   MType = "gauge"
)

func (m MType) IsValid() bool {
	switch m {
	case Counter, Gauge:
		return true
	default:
		return false
	}
}
