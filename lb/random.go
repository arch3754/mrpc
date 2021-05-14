package lb

import "github.com/valyala/fastrand"

type RandomLoadBalancer struct {
	Addrs []string
}

func (lb *RandomLoadBalancer) Get() string {
	return lb.Addrs[fastrand.Uint32n(uint32(len(lb.Addrs)))]

}
func (lb *RandomLoadBalancer) UpdateAddrs(addrs []string) {
	lb.Addrs = addrs[:]
}