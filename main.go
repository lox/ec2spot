package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aybabtme/uniplot/histogram"
	"github.com/lox/ec2spot/data"
	"github.com/lox/ec2spot/fetcher"
	"github.com/lox/ec2spot/timerange"
)

func main() {
	daysFlag := flag.Int("days", 7, "How many days to go back")
	instanceFlag := flag.String("instance", "c4.large", "Show results for a particular instance type, or multiple comma delimited")
	productFlag := flag.String("product", "Linux/UNIX (Amazon VPC)", "Show results for a particular product type")
	regionFlag := flag.String("region", "us-east-1", "Show results for a particular region")
	azsFlag := flag.String("azs", "", "Only include specific availability zones (e.g a,b,c)")
	concurrencyFlag := flag.Int("concurrency", 10, "How many concurrent AWS requests to make")
	maxBidFlag := flag.Float64("max-bid", 0, "Maximum bid to make in estimates")
	flag.Parse()

	regions := strings.Split(*regionFlag, ",")
	azs := parseAvailabilityZones(regions, *azsFlag)
	instanceTypes := strings.Split(*instanceFlag, ",")

	prices, err := runAnalysis(context.Background(), analysisParams{
		InstanceTypes:     instanceTypes,
		Regions:           regions,
		AvailabilityZones: azs,
		Concurrency:       *concurrencyFlag,
		Product:           *productFlag,
		Days:              *daysFlag,
	})

	if err != nil {
		log.Fatal(err)
	}

	for _, region := range regions {
		for _, instanceType := range instanceTypes {
			foundAZs := prices.AvailabilityZones()
			sliced := prices.
				ByRegion(region).
				ByInstanceType(instanceType)

			fmt.Printf("%-20s%s\n", "Region:", region)
			fmt.Printf("%-20s%s\n", "Instance Type:", instanceType)

			info, err := data.GetInstanceTypeInfo(region, instanceType)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("%-20s$%.6f\n", "On-Demand Price:", info.Price)

			fmt.Printf("\nAll Availability Zones %s\n", strings.Join(foundAZs, ","))
			showHistograph(sliced)

			for _, az := range foundAZs {
				fmt.Printf("\nAvailability Zone %s\n", az)
				showHistograph(sliced.ByAvailabilityZone(az))
			}

			estimateCost(costEstimateParams{
				Days:         *daysFlag,
				InstanceInfo: info,
				Prices:       sliced,
				MaxBid:       *maxBidFlag,
			})
		}
	}
}

type analysisParams struct {
	Range             timerange.Range
	InstanceTypes     []string
	Regions           []string
	AvailabilityZones []string
	Product           string
	Days              int
	Concurrency       int
}

type costEstimateParams struct {
	Days         int
	InstanceInfo data.InstanceTypeInfo
	Prices       data.SpotPriceSlice
	MaxBid       float64
}

func estimateCost(params costEstimateParams) {
	var totalSpotCost, totalOnDemandCost float64
	var timesOutbid int

	tr := timerange.DaysAgo(time.Now(), params.Days)
	hours := tr.Split(time.Hour)
	maxBid := params.Prices.Max()

	totalOnDemandCost = params.InstanceInfo.Price * float64(len(hours))

	if params.MaxBid > 0 && maxBid > params.MaxBid {
		maxBid = params.MaxBid
	}

	for _, bucket := range params.Prices.Buckets(hours) {
		outBid := true

		for _, az := range bucket.Prices.AvailabilityZones() {
			if maxBid >= bucket.Prices.ByAvailabilityZone(az).Max() {
				outBid = false
				break
			}
		}

		if !outBid {
			totalSpotCost += bucket.Prices.Max()
		} else {
			timesOutbid++
		}
	}

	fmt.Println("")
	fmt.Printf("Time range is %d days, or %d hours\n", params.Days, len(hours))
	fmt.Printf("At on-demand price of $%.4g (across all azs): $%.4g\n",
		params.InstanceInfo.Price, totalOnDemandCost)
	fmt.Printf("At maximum spot bid of $%.4g (across all azs): $%.4g (%%%.2f of on-demand)\n",
		maxBid, totalSpotCost, ((totalOnDemandCost-totalSpotCost)/totalOnDemandCost)*100)
	fmt.Printf("Time outbid: %d\n", timesOutbid)
}

func runAnalysis(ctx context.Context, params analysisParams) (data.SpotPriceSlice, error) {
	results, g := fetcher.BatchFetch(ctx, params.Concurrency, fetcher.BatchFetchSpec{
		InstanceTypes:     params.InstanceTypes,
		Regions:           params.Regions,
		AvailabilityZones: params.AvailabilityZones,
		Product:           params.Product,
		Days:              params.Days,
	})

	prices := data.SpotPriceSlice{}
	for price := range results {
		prices = append(prices, price)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return prices, nil
}

var reAz = regexp.MustCompile(`([a-z]+\-[a-z]+-[0-9])([a-z])?$`)

func parseAvailabilityZones(regions []string, azsFlag string) []string {
	azs := []string{}

	for _, s := range strings.Split(azsFlag, ",") {
		if s != "" {
			azs = append(azs, regions[0]+s)
		}
	}

	return azs
}

func formatPrice(v float64) string {
	return fmt.Sprintf("%.6g", v)
}

func showHistograph(prices data.SpotPriceSlice) error {
	bins := 3
	data := []float64{}

	for _, p := range prices {
		data = append(data, p.Price)
	}

	hist := histogram.Hist(bins, data)
	maxWidth := 40
	return histogram.Fprintf(os.Stdout, hist, histogram.Linear(maxWidth), formatPrice)
}
