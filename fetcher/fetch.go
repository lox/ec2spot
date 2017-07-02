package fetcher

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/lox/ec2spot/data"
	"github.com/lox/ec2spot/timerange"
)

var (
	clients     = map[string]*ec2.EC2{}
	clientsLock sync.Mutex
)

type FetchSpec struct {
	Region             string
	Start, End         time.Time
	InstanceType       string
	ProductDescription string
	AvailabilityZone   string
}

func ec2Client(region string) (*ec2.EC2, error) {
	clientsLock.Lock()
	defer clientsLock.Unlock()

	svc, ok := clients[region]
	if !ok {
		config := aws.NewConfig()
		sess, err := session.NewSession(config.WithRegion(region))
		if err != nil {
			return nil, err
		}

		svc = ec2.New(sess)
		clients[region] = svc
	}

	return svc, nil
}

func Fetch(spec FetchSpec) (data.SpotPriceSlice, error) {
	svc, err := ec2Client(spec.Region)
	if err != nil {
		return nil, err
	}

	log.Printf("Fetching %s", timerange.Range{spec.Start, spec.End})

	prices := data.SpotPriceSlice{}
	params := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       aws.StringSlice([]string{spec.InstanceType}),
		ProductDescriptions: aws.StringSlice([]string{spec.ProductDescription}),
		StartTime:           aws.Time(spec.Start),
		EndTime:             aws.Time(spec.End),
	}

	if spec.AvailabilityZone != "" {
		params.AvailabilityZone = aws.String(spec.AvailabilityZone)
	}

	err = svc.DescribeSpotPriceHistoryPages(params,
		func(page *ec2.DescribeSpotPriceHistoryOutput, lastPage bool) bool {
			for _, price := range page.SpotPriceHistory {
				priceVal, _ := strconv.ParseFloat(*price.SpotPrice, 64)
				prices = append(prices, data.SpotPrice{
					InstanceType:     *price.InstanceType,
					AvailabilityZone: *price.AvailabilityZone,
					Price:            float64(priceVal),
					Timestamp:        *price.Timestamp,
				})
			}
			return lastPage
		})

	return data.SpotPriceSlice(prices), err
}
