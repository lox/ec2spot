package data

import (
	"fmt"
	"time"

	"github.com/lox/ec2spot/timerange"
)

type SpotPrice struct {
	Region           string
	InstanceType     string
	AvailabilityZone string
	Price            float64
	Timestamp        time.Time
}

type SpotPriceSlice []SpotPrice

func (r SpotPriceSlice) Max() float64 {
	var max float64

	for _, p := range r {
		if p.Price > max {
			max = p.Price
		}
	}

	return max
}

func (r SpotPriceSlice) Min() float64 {
	min := r[0].Price

	for _, p := range r {
		if p.Price < min {
			min = p.Price
		}
	}

	return min
}

func (r SpotPriceSlice) Average() float64 {
	var total float64

	for _, p := range r {
		total += p.Price
	}

	return total / float64(len(r))
}

func (r SpotPriceSlice) Subset(tr timerange.Range) SpotPriceSlice {
	subset := SpotPriceSlice{}

	for _, sp := range r {
		if tr.Contains(sp.Timestamp) {
			subset = append(subset, sp)
		}
	}

	return subset
}

func (r SpotPriceSlice) ByAvailabilityZone(az string) SpotPriceSlice {
	subset := SpotPriceSlice{}

	for _, sp := range r {
		if sp.AvailabilityZone == az {
			subset = append(subset, sp)
		}
	}

	return subset
}

func (r SpotPriceSlice) AvailabilityZones() []string {
	zones := []string{}
	zoneMap := map[string]struct{}{}

	for _, sp := range r {
		if _, ok := zoneMap[sp.AvailabilityZone]; !ok {
			zoneMap[sp.AvailabilityZone] = struct{}{}
			zones = append(zones, sp.AvailabilityZone)
		}
	}

	return zones
}

func (r SpotPriceSlice) String() string {
	return fmt.Sprintf("Price range (%d points): Min %.5f Max %.5f Avg %.5f",
		len(r), r.Min(), r.Max(), r.Average(),
	)
}
