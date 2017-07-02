package main

import (
	"context"
	"flag"
	"log"
	"os"
	"regexp"
	"strings"

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
	bidFlag := flag.Float64("bid", 0, "Maximum bid to make in estimates")
	splitAzsFlag := flag.Bool("split-azs", false, "Whether to show availability zones individually")
	flag.Parse()

	regions := strings.Split(*regionFlag, ",")
	azs := parseAvailabilityZones(regions, *azsFlag)

	prices, err := runAnalysis(context.Background(), analysisParams{
		InstanceTypes:     strings.Split(*instanceFlag, ","),
		Regions:           regions,
		AvailabilityZones: azs,
		Concurrency:       *concurrencyFlag,
		Product:           *productFlag,
		Days:              *daysFlag,
		MaxBid:            *bidFlag,
		SplitZones:        *splitAzsFlag,
	})

	if err != nil {
		log.Fatal(err)
	}

	showHistograph(prices)

	// log.Printf("%#v", prices)
}

type analysisParams struct {
	Range             timerange.Range
	InstanceTypes     []string
	Regions           []string
	AvailabilityZones []string
	Product           string
	Days              int
	Concurrency       int
	MaxBid            float64
	SplitZones        bool
}

// func estimatePrice() {
// 	var maxBid, totalSpotCost, totalOnDemandCost float64
// 	var hours, outbid, partialOutbid int64

// 	for _, hour := range params.Range.Split(time.Hour) {
// 		var outbidCount int
// 		for _, az := range azs {
// 			hourPrices := results.ByAvailabilityZone(az).Subset(hour)
// 			hourMaxBid := hourPrices.Max()
// 			bid := hourMaxBid

// 			// if params.MaxBid > 0 && hourMaxBid > params.MaxBid {
// 			// 	bid = params.bid
// 			// }

// 			if len(hourPrices) > 0 {
// 				totalSpotCost += hourPrices.Max()
// 			} else {
// 				totalSpotCost += onDemandPrice
// 			}

// 			totalOnDemandCost += onDemandPrice
// 		}
// 	}

// 	fmt.Printf("\nSpot price for %d hours would be $%.2f (~$%.5f hourly) vs $%.2f on-demand (%.2f%% difference)\n\n",
// 		hours,
// 		totalSpotCost,
// 		totalSpotCost/float64(hours),
// 		totalOnDemandCost,
// 		((totalOnDemandCost-totalSpotCost)/totalOnDemandCost)*100,
// 	)
// }

// fmt.Printf("Region:             %s\n", params.Region)

// if len(params.AvailabilityZones) > 0 {
// 	fmt.Printf("Availability Zones: %s\n", strings.Join(params.AvailabilityZones, ", "))
// }

// data, err := ec2instancesinfo.Data()
// if err != nil {
// 	return err
// }

// var onDemandPrice float64

// for _, i := range *data {
// 	if i.InstanceType == params.InstanceType {
// 		fmt.Printf("Instance Type:      %s\n", i.PrettyName)
// 		fmt.Printf("VCPU:               %d\n", i.VCPU)
// 		fmt.Printf("Memory:             %.2f\n", i.Memory)
// 		fmt.Printf("On-Demand Price:    %.6f\n\n", i.Pricing[params.Region].Linux.OnDemand)

// 		// onDemandPrice = i.Pricing[params.Region].Linux.OnDemand
// 	}
// }

// azs := results.AvailabilityZones()
// fmt.Printf("All Availability Zones\n")

// if err = showHistograph(results); err != nil {
// 	return err
// }

// if params.SplitZones {
// 	for _, az := range azs {
// 		fmt.Printf("\nAvailability Zone %s\n", az)

// 		if err = showHistograph(results.ByAvailabilityZone(az)); err != nil {
// 			return err
// 		}
// 	}
// }
// return nil

func runAnalysis(ctx context.Context, params analysisParams) (data.SpotPriceSlice, error) {
	prices, g := fetcher.BatchFetch(ctx, params.Concurrency, fetcher.BatchFetchSpec{
		InstanceTypes:     params.InstanceTypes,
		Regions:           params.Regions,
		AvailabilityZones: params.AvailabilityZones,
		Product:           params.Product,
		Days:              params.Days,
	})

	results := data.SpotPriceSlice{}
	for price := range prices {
		results = append(results, price)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

var reAz = regexp.MustCompile(`([a-z]+\-[a-z]+-[0-9])([a-z])?$`)

func parseAvailabilityZones(regions []string, azsFlag string) []string {
	azs := []string{}

	for _, s := range strings.Split(azsFlag, ",") {
		log.Printf("AZ: %#v Regions: %#v", s, regions)
		if s != "" {
			azs = append(azs, regions[0]+s)
		}
	}

	return azs
}

func showHistograph(prices data.SpotPriceSlice) error {
	bins := 4
	data := []float64{}

	for _, p := range prices {
		data = append(data, p.Price)
	}

	hist := histogram.Hist(bins, data)
	maxWidth := 40
	return histogram.Fprint(os.Stdout, hist, histogram.Linear(maxWidth))
}
