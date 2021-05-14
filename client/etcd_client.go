package client

import (
	"context"
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
	etcdClient, err := clientv3.New(clientv3.Config{Endpoints: etcdAddr, DialTimeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	c := &EtcdClient{etcdClient: etcdClient, prefix: prefix, option: option,serverConnPool:make(map[string]*client)}
	if err = c.init(); err != nil {
		return nil, err
	}
	c.lb = c.selectLBMode(option.LoadBalance)
	return c, nil
}
func (c *EtcdClient) selectLBMode(lbr int) lb.LoadBalancer {
	switch lbr {
	case lb.RoundRobin:
		return &lb.RoundRobinLoadBalancer{Addrs: c.serverKeyList[:]}
	case lb.Random:
		return &lb.RandomLoadBalancer{Addrs: c.serverKeyList[:]}
	default:
		return &lb.RoundRobinLoadBalancer{Addrs: c.serverKeyList[:]}
	}
}
func (c *EtcdClient) Call(ctx context.Context, path, method string, arg, reply interface{}) error {
	c.lock.RLock()
	cli := c.serverConnPool[c.lb.Get()]
	c.lock.RUnlock()
	return cli.SyncCall(ctx, path, method, arg, reply)
}
func (c *EtcdClient) AsyncCall(ctx context.Context, path, method string, arg, reply interface{}) *Caller {
	c.lock.RLock()
	cli := c.serverConnPool[c.lb.Get()]
	c.lock.RUnlock()
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
	arr := strings.Split(val, "@")
	if len(arr) != 2 {
		log.Rlog.Warn("ignore server %v connect.", key)
		return
	}
	err := cli.Connect(arr[0], arr[1])
	if err != nil {
		log.Rlog.Warn("server %v connect failed:%v", key, err)
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.serverKeyList = append(c.serverKeyList, key)
	c.serverConnPool[key] = cli

}

func (c *EtcdClient) delServiceList(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if v, ok := c.serverConnPool[key]; ok {
		v.Close()
	}
	delete(c.serverConnPool, key)
	for k, v := range c.serverKeyList {
		if v == key {
			c.serverKeyList = append(c.serverKeyList[:k], c.serverKeyList[:k+1]...)
			return
		}
	}
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