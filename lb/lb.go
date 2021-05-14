package lb

const (
	RoundRobin = iota
	Random
	CpuWeight
	ConsistentHash
)

type LoadBalancer interface {
	Get() string
	UpdateAddrs(addrs []string)
}