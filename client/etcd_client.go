package client

import (
	"context"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/clientv3"
	"math/rand"
	"strings"
	"sync"
	"time"
)

func NewEtcdClient() {

}

type EtcdDiscovery struct {
	etcdClient *clientv3.Client
	serverList map[string]string
	lock       sync.Mutex
}

func NewEtcdDiscovery(addr []string, prefix string) (*EtcdDiscovery, error) {
	cli := &EtcdDiscovery{serverList: make(map[string]string)}
	conf := clientv3.Config{Endpoints: addr, DialTimeout: 5 * time.Second}
	var err error
	cli.etcdClient, err = clientv3.New(conf)
	if err != nil {
		return nil, err
	}
	resp, err := cli.etcdClient.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Kvs != nil {
		for i := range resp.Kvs {
			if v := resp.Kvs[i].Value; v != nil {
				cli.setServiceList(string(resp.Kvs[i].Key), string(resp.Kvs[i].Value))
			}
		}
	}

	go func() {
		rch := cli.etcdClient.Watch(context.Background(), prefix, clientv3.WithPrefix())
		for wResp := range rch {
			for _, ev := range wResp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					cli.setServiceList(string(ev.Kv.Key), string(ev.Kv.Value))
				case mvccpb.DELETE:
					cli.delServiceList(string(ev.Kv.Key))
				}
			}
		}
	}()

	return cli, nil
}

func (d *EtcdDiscovery) setServiceList(key, val string) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.serverList[key] = val
}

func (d *EtcdDiscovery) delServiceList(key string) {
	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.serverList, key)
}
func (d *EtcdDiscovery) GetServerList() *[]string {
	var addrs []string
	for _, v := range d.serverList {
		addrs = append(addrs, v)
	}
	return &addrs
}

func (d *EtcdDiscovery) ServiceOne(prefix string) string {
	d.lock.Lock()
	defer d.lock.Unlock()

	addrArr := make([]string, 0, 16)
	for k, v := range d.serverList {
		if strings.HasPrefix(k, prefix) {
			addrArr = append(addrArr, v)
		}
	}

	if len(addrArr) == 0 {
		return ""
	}

	// 随机获取一个服务
	r := rand.New(rand.NewSource(time.Now().Unix()))
	return addrArr[r.Intn(len(addrArr))]
}