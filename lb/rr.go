package lb

type RoundRobinLoadBalancer struct {
	Addrs []string
	seq   int
}

func (lb *RoundRobinLoadBalancer) Get() string {
	i := lb.seq % len(lb.Addrs)
	lb.seq = i + 1

	return lb.Addrs[i]
}
func (lb *RoundRobinLoadBalancer) UpdateAddrs(addrs []string) {
	lb.Addrs = addrs
}
