ec2spot
=======

A cli tool for sampling recent EC2 spot pricing data and showing a histogram of the results. An analysis of relative cost for that period is also performed based on the actual per-hour pricing.

```bash
$ ec2spot -days 30 -instance m4.large -region us-east-1
Instance Type:    M4 Large
VCPU:             2
Memory:           8.00
On-Demand Price:  0.108000

0.0134-0.4289  100%     ████████████████████▏  39791
0.4289-0.8445  0%       ▏                      
0.8445-1.26    0.0126%  ▏                      5

Spot price for 742 hours would be $27.77 (~$0.03743 hourly) vs $80.14 on-demand (65.35% difference)
```