package fetcher

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/lox/ec2spot/data"
	"github.com/lox/ec2spot/timerange"
)

const chunkSize = time.Hour * 8

type BatchFetchSpec struct {
	InstanceTypes     []string
	Regions           []string
	AvailabilityZones []string
	Product           string
	Days              int
}

func (params BatchFetchSpec) ToFetchSpecs(chunkSize time.Duration) []FetchSpec {
	var specs = []FetchSpec{}
	var forEachAz = func(azs []string, f func(string)) {
		if len(azs) == 0 {
			f("")
			return
		}
		for _, az := range azs {
			f(az)
		}
	}

	for _, region := range params.Regions {
		for _, instanceType := range params.InstanceTypes {
			forEachAz(params.AvailabilityZones, func(az string) {
				for _, r := range timerange.DaysAgo(time.Now(), params.Days).Split(chunkSize) {
					specs = append(specs, FetchSpec{
						Region:             region,
						Start:              r[0],
						End:                r[1],
						ProductDescription: params.Product,
						InstanceType:       instanceType,
						AvailabilityZone:   az,
					})
				}
			})
		}
	}

	return specs
}

func BatchFetch(ctx context.Context, concurrency int, spec BatchFetchSpec) (chan data.SpotPrice, *errgroup.Group) {
	g, ctx := errgroup.WithContext(ctx)
	specs := make(chan FetchSpec, 100)

	// load up a channel with specs to fetch
	g.Go(func() error {
		defer close(specs)
		for _, spec := range spec.ToFetchSpecs(chunkSize) {
			select {
			case specs <- spec:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	// start a fixed number of goroutines to send aws requests
	prices := make(chan data.SpotPrice)
	for i := 0; i < concurrency; i++ {
		g.Go(func() error {
			for spec := range specs {
				result, err := Fetch(spec)
				if err != nil {
					return err
				}
				for _, price := range result {
					select {
					case prices <- price:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(prices)
	}()

	return prices, g
}
