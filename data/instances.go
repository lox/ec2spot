package data

import (
	ec2instancesinfo "github.com/cristim/ec2-instances-info"
)

var data *ec2instancesinfo.InstanceData

func init() {
	var err error
	data, err = ec2instancesinfo.Data()
	if err != nil {
		panic(err)
	}
}

type InstanceTypeInfo struct {
	PrettyName string
	VCPU       int
	Memory     float32
	Price      float64
}

func GetInstanceTypeInfo(region, instanceType string) (InstanceTypeInfo, error) {
	for _, i := range *data {
		if i.InstanceType == instanceType {
			return InstanceTypeInfo{
				PrettyName: i.PrettyName,
				VCPU:       i.VCPU,
				Memory:     i.Memory,
				Price:      i.Pricing[region].Linux.OnDemand,
			}, nil
		}
	}

	return InstanceTypeInfo{}, nil
}
