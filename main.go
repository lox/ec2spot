package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aybabtme/uniplot/histogram"
	ec2instancesinfo "github.com/cristim/ec2-instances-info"
	"github.com/lox/ec2spot/timerange"
	"golang.org/x/sync/errgroup"
)

func formatSpotPrice(p *ec2.SpotPrice) {
	fmt.Printf("%s %s %s %s\n",
		*p.AvailabilityZone,
		*p.InstanceType,
		*p.SpotPrice,
		p.Timestamp.Format(time.Stamp),
	)
}

func showHistograph(prices spotPriceRange) error {
	bins := 3
	data := []float64{}

	for _, p := range prices {
		data = append(data, p.Price)
	}

	hist := histogram.Hist(bins, data)
	maxWidth := 20
	return histogram.Fprint(os.Stdout, hist, histogram.Linear(maxWidth))
}

type spotPriceSpec struct {
	Region             string
	Start, End         time.Time
	InstanceType       string
	ProductDescription string
}

type spotPrice struct {
	Region       string
	InstanceType string
	Zone         string
	Price        float64
	Timestamp    time.Time
}

type spotPriceRange []spotPrice

func (r spotPriceRange) Max() float64 {
	var max float64

	for _, p := range r {
		if p.Price > max {
			max = p.Price
		}
	}

	return max
}

func (r spotPriceRange) Min() float64 {
	min := r[0].Price

	for _, p := range r {
		if p.Price < min {
			min = p.Price
		}
	}

	return min
}

func (r spotPriceRange) Average() float64 {
	var total float64

	for _, p := range r {
		total += p.Price
	}

	return total / float64(len(r))
}

func (r spotPriceRange) Subset(tr timerange.Range) spotPriceRange {
	subset := spotPriceRange{}

	for _, sp := range r {
		if tr.Contains(sp.Timestamp) {
			subset = append(subset, sp)
		}
	}

	return subset
}

func (r spotPriceRange) String() string {
	return fmt.Sprintf("Price range (%d points): Min %.5f Max %.5f Avg %.5f",
		len(r), r.Min(), r.Max(), r.Average(),
	)
}

func readSpotPrices(client *ec2.EC2, spec spotPriceSpec) (spotPriceRange, error) {
	prices := spotPriceRange{}
	params := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       aws.StringSlice([]string{spec.InstanceType}),
		ProductDescriptions: aws.StringSlice([]string{spec.ProductDescription}),
		StartTime:           aws.Time(spec.Start),
		EndTime:             aws.Time(spec.End),
	}

	err := client.DescribeSpotPriceHistoryPages(params,
		func(page *ec2.DescribeSpotPriceHistoryOutput, lastPage bool) bool {
			for _, price := range page.SpotPriceHistory {
				priceVal, _ := strconv.ParseFloat(*price.SpotPrice, 64)
				prices = append(prices, spotPrice{
					InstanceType: *price.InstanceType,
					Zone:         *price.AvailabilityZone,
					Price:        float64(priceVal),
					Timestamp:    *price.Timestamp,
				})
			}
			return lastPage
		})
	return spotPriceRange(prices), err
}

func main() {
	debugFlag := flag.Bool("debug", false, "Show debugging output")
	daysFlag := flag.Int("days", 7, "How many days to go back")
	instanceFlag := flag.String("instance", "c4.large", "Show results for a particular instance type")
	productFlag := flag.String("product", "Linux/UNIX (Amazon VPC)", "Show results for a particular product type")
	regionFlag := flag.String("region", "us-east-1", "Show results for a particular region")
	concurrencyFlag := flag.Int("concurrency", 12, "How many concurrent AWS requests to make")
	flag.Parse()

	config := aws.NewConfig()
	if *debugFlag {
		config = config.WithLogLevel(
			aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors,
		)
	}

	sess, err := session.NewSession(config.WithRegion(*regionFlag))
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
		return
	}

	dayRange := timerange.Range{
		timerange.StartOfDay(time.Now()).AddDate(0, 0, *daysFlag*-1),
		time.Now(),
	}

	svc := ec2.New(sess)

	g, ctx := errgroup.WithContext(context.Background())
	specs := make(chan spotPriceSpec, 100)

	// generate the specs for the different time range chunks
	g.Go(func() error {
		defer close(specs)
		for _, day := range dayRange.Days() {
			for _, r := range day.Split(time.Hour * 4) {
				spec := spotPriceSpec{
					Region:             *regionFlag,
					Start:              r[0],
					End:                r[1],
					ProductDescription: *productFlag,
					InstanceType:       *instanceFlag,
				}
				select {
				case specs <- spec:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	})

	// start a fixed number of goroutines to send aws requests
	prices := make(chan spotPrice)
	for i := 0; i < *concurrencyFlag; i++ {
		g.Go(func() error {
			for spec := range specs {
				if *debugFlag {
					log.Printf("Processing %v", spec)
				}
				result, err := readSpotPrices(svc, spec)
				if err != nil {
					return err
				}
				if *debugFlag {
					log.Printf("Got back %d prices", len(result))
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

	results := spotPriceRange{}
	for price := range prices {
		results = append(results, price)
	}

	if err = g.Wait(); err != nil {
		log.Fatal(err)
	}

	data, err := ec2instancesinfo.Data()
	if err != nil {
		log.Fatal(err)
	}

	var onDemandPrice float64

	for _, i := range *data {
		if i.InstanceType == *instanceFlag {
			fmt.Printf("Instance Type:    %s\n", i.PrettyName)
			fmt.Printf("VCPU:             %d\n", i.VCPU)
			fmt.Printf("Memory:           %.2f\n", i.Memory)
			fmt.Printf("On-Demand Price:  %.6f\n\n", i.Pricing[*regionFlag].Linux.OnDemand)

			onDemandPrice = i.Pricing[*regionFlag].Linux.OnDemand
		}
	}

	if err = showHistograph(results); err != nil {
		log.Fatal(err)
	}

	var totalSpotCost, totalOnDemandCost float64
	var hours int64

	for _, hour := range dayRange.Split(time.Hour) {
		hourPrices := results.Subset(hour)
		hours++

		if len(hourPrices) > 0 {
			totalSpotCost += hourPrices.Max()
		} else {
			totalSpotCost += onDemandPrice
		}

		totalOnDemandCost += onDemandPrice
	}

	fmt.Printf("\nSpot price for %d hours would be $%.2f (~$%.5f hourly) vs $%.2f on-demand (%.2f%% difference)\n\n",
		hours,
		totalSpotCost,
		totalSpotCost/float64(hours),
		totalOnDemandCost,
		((totalOnDemandCost-totalSpotCost)/totalOnDemandCost)*100,
	)
}
