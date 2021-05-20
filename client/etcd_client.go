package client

import (
	"context"
	"fmt"
	"github.com/arch3754/mrpc/lb"
	"github.com/arch3754/mrpc/log"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/clientv3"
	"strings"
	"sync"
	"time"
)

type EtcdClient struct {
	option         *Option
	prefix         string
	etcdClient     *clientv3.Client
	serverConnPool map[string]*client
	serverKeyList  []string
	lock           sync.RWMutex
	lb             lb.LoadBalancer
}

func NewEtcdClient(etcdAddr []string, prefix string, option *Option) (*EtcdClient, error) {
	if option == nil {
		option = DefaultOption
	}
	etcdClient, err := clientv3.New(clientv3.Config{Endpoints: etcdAddr, DialTimeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	lbr, ok := lb.LoadBalancerMap[option.LoadBalance]
	if !ok {
		lbr = &lb.RoundRobinLoadBalancer{}
	}
	c := &EtcdClient{
		etcdClient:     etcdClient,
		prefix:         prefix,
		option:         option,
		lb:             lbr,
		serverConnPool: make(map[string]*client),
	}
	if err = c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *EtcdClient) getClient() (*client, error) {
	if len(c.serverConnPool) == 0 {
		return nil, fmt.Errorf("not available service")
	}
	if !c.option.Breaker.Ready() {
		return nil, fmt.Errorf("breaker ready")
	}
	key := c.lb.Get()
	c.lock.RLock()
	defer c.lock.RUnlock()
	cli := c.serverConnPool[key]
	if cli.isClose {
		network, addr, err := c.buildAddress(key)
		err = cli.Connect(network, addr)
		if err != nil {
			c.option.Breaker.Fail()
			return nil, err
		}
		c.option.Breaker.Success()
		c.serverConnPool[key] = cli
	}

	return cli, nil
}
func (c *EtcdClient) buildAddress(key string) (string, string, error) {
	arr := strings.Split(key, "@")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("address parse failed")
	}
	return arr[0], arr[1], nil
}
func (c *EtcdClient) Call(ctx context.Context, path, method string, arg, reply interface{}) error {
	cli, err := c.getClient()
	if err != nil {
		return err
	}
	return cli.SyncCall(ctx, path, method, arg, reply)
}
func (c *EtcdClient) AsyncCall(ctx context.Context, path, method string, arg, reply interface{}) *Caller {
	cli, err := c.getClient()
	if err != nil {
		var caller = &Caller{
			Path:   path,
			Method: method,
			Error:  err,
			Done:   make(chan *Caller, 1),
		}
		caller.Done <- caller
		return caller
	}

	return cli.AsyncCall(ctx, path, method, arg, reply)
}
func (c *EtcdClient) init() error {
	resp, err := c.etcdClient.Get(context.Background(), c.prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	if resp != nil && resp.Kvs != nil {
		for i := range resp.Kvs {
			if v := resp.Kvs[i].Value; v != nil {
				c.setServiceList(string(resp.Kvs[i].Key), string(resp.Kvs[i].Value))
			}
		}
	}
	return nil
}

func (c *EtcdClient) setServiceList(key, val string) {
	cli := NewClient(c.option)
	network, addr, err := c.buildAddress(val)

	err = cli.Connect(network, addr)
	if err != nil {
		log.Rlog.Warn("server %v connect failed:%v", val, err)
		return
	}
	c.lock.Lock()
	c.serverKeyList = append(c.serverKeyList, val)
	c.serverConnPool[val] = cli
	c.lb.UpdateAddrs(c.serverKeyList)
	c.lock.Unlock()

}

func (c *EtcdClient) delServiceList(key string) {
	c.lock.Lock()
	if v, ok := c.serverConnPool[key]; ok {
		_ = v.Close()
	}
	delete(c.serverConnPool, key)
	for k, v := range c.serverKeyList {
		if v == key {
			c.serverKeyList = append(c.serverKeyList[:k], c.serverKeyList[:k+1]...)
			break
		}
	}
	c.lb.UpdateAddrs(c.serverKeyList)
	c.lock.Unlock()

}
func (c *EtcdClient) watch() {
	go func() {
		rch := c.etcdClient.Watch(context.Background(), c.prefix, clientv3.WithPrefix())
		for wResp := range rch {
			for _, ev := range wResp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					c.setServiceList(string(ev.Kv.Key), string(ev.Kv.Value))
				case mvccpb.DELETE:
					c.delServiceList(string(ev.Kv.Key))
				}
			}
		}
	}()
}