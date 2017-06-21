package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
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

type analysisParams struct {
	Range        timerange.Range
	InstanceType string
	Region       string
	Product      string
	Days         int
	Concurrency  int
}

func runAnalysis(svc *ec2.EC2, params analysisParams) error {
	g, ctx := errgroup.WithContext(context.Background())
	specs := make(chan spotPriceSpec, 100)

	// generate the specs for the different time range chunks
	g.Go(func() error {
		defer close(specs)
		for _, day := range params.Range.Days() {
			for _, r := range day.Split(time.Hour * 5) {
				spec := spotPriceSpec{
					Region:             params.Region,
					Start:              r[0],
					End:                r[1],
					ProductDescription: params.Product,
					InstanceType:       params.InstanceType,
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
	for i := 0; i < params.Concurrency; i++ {
		g.Go(func() error {
			for spec := range specs {
				result, err := readSpotPrices(svc, spec)
				if err != nil {
					return err
				}
				if len(result) >= 1000 {
					log.Printf("Warning: results are clipped at 1000")
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

	if err := g.Wait(); err != nil {
		return err
	}

	data, err := ec2instancesinfo.Data()
	if err != nil {
		return err
	}

	var onDemandPrice float64

	for _, i := range *data {
		if i.InstanceType == params.InstanceType {
			fmt.Printf("Instance Type:    %s\n", i.PrettyName)
			fmt.Printf("VCPU:             %d\n", i.VCPU)
			fmt.Printf("Memory:           %.2f\n", i.Memory)
			fmt.Printf("On-Demand Price:  %.6f\n\n", i.Pricing[params.Region].Linux.OnDemand)

			onDemandPrice = i.Pricing[params.Region].Linux.OnDemand
		}
	}

	if err = showHistograph(results); err != nil {
		return err
	}

	var totalSpotCost, totalOnDemandCost float64
	var hours int64

	for _, hour := range params.Range.Split(time.Hour) {
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

	return nil
}

func main() {
	daysFlag := flag.Int("days", 7, "How many days to go back")
	instanceFlag := flag.String("instance", "c4.large", "Show results for a particular instance type, or multiple comma delimited")
	productFlag := flag.String("product", "Linux/UNIX (Amazon VPC)", "Show results for a particular product type")
	regionFlag := flag.String("region", "us-east-1", "Show results for a particular region")
	concurrencyFlag := flag.Int("concurrency", 12, "How many concurrent AWS requests to make")
	flag.Parse()

	config := aws.NewConfig()
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

	for _, instanceType := range strings.Split(*instanceFlag, ",") {
		params := analysisParams{
			InstanceType: instanceType,
			Region:       *regionFlag,
			Concurrency:  *concurrencyFlag,
			Product:      *productFlag,
			Days:         *daysFlag,
			Range:        dayRange,
		}

		if err := runAnalysis(svc, params); err != nil {
			log.Fatal(err)
		}
	}
}
