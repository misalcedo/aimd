# AIMD - Additive increase/multiplicative decrease
AIMD is a congestion control algorithm used in TCP.
It is a simple algorithm that increases the congestion window size by 1 MSS (Maximum Segment Size) every RTT (Round Trip Time) until a packet loss is detected.
When a packet loss is detected, the congestion window size is halved.
The congestion window size is then increased by 1 MSS every RTT until another packet loss is detected.
This process is repeated until the congestion window size reaches the maximum value.

See https://en.wikipedia.org/wiki/Additive_increase/multiplicative_decrease

## Acknowledgement
This code is based a fork of [golang.org/x/sync/semaphore#Weighted](https://pkg.go.dev/golang.org/x/sync@v0.7.0/semaphore#Weighted).
All code from the original implementation maintains its original license.
The semaphore implementation is extended to support AIMD.