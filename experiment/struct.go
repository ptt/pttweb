package experiment

import (
	"encoding/json"
	"fmt"
)

const percentBase = uint64(10000)

type Percent struct {
	threshold uint64
}

func (p *Percent) UnmarshalJSON(b []byte) error {
	var pct float64
	if err := json.Unmarshal(b, &pct); err != nil {
		return err
	}
	if pct < 0 {
		return fmt.Errorf("percent is negative: %v", pct)
	}
	p.threshold = uint64(pct * float64(percentBase) / 100)
	return nil
}

type OptIn struct {
	OptIn Percent
}

func (o *OptIn) Enabled(val uint64) bool {
	return val%percentBase < o.OptIn.threshold
}
