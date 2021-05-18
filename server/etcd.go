package server

import (
	"context"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/util"
	"go.etcd.io/etcd/clientv3"
)

type EtcdPlugin struct {
	client        *clientv3.Client
	leaseID       clientv3.LeaseID //租约ID
	config        *EtcdConfig
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
}
type EtcdConfig struct {
	BasePath      string
	Lease         int64
	RpcServerAddr string
	EtcdConf      *clientv3.Config
}

func NewEtcdPlugin(cfg *EtcdConfig) (Plugin, error) {
	if cfg.Lease == 0 {
		cfg.Lease = 5
	}
	if len(cfg.BasePath) == 0 {
		cfg.BasePath = util.DefaultRpcBasePath
	}
	c, err := clientv3.New(*cfg.EtcdConf)
	if err != nil {
		return nil, err
	}

	return &EtcdPlugin{
		config: cfg,
		client: c,
	}, nil
}

func (p *EtcdPlugin) ServiceRegister() error {
	//设置租约时间
	resp, err := p.client.Grant(context.Background(), p.config.Lease)
	if err != nil {
		return err
	}
	//注册服务并绑定租约
	_, err = p.client.Put(context.Background(), p.config.BasePath+"/"+p.config.RpcServerAddr, p.config.RpcServerAddr, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}
	//设置续租 定期发送需求请求
	leaseRespChan, err := p.client.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		return err
	}
	p.leaseID = resp.ID
	p.keepAliveChan = leaseRespChan
	go p.listenLeaseRespChan()
	//log.Printf("Put key:%s  val:%s  success!", s.key, s.val)
	return nil
}

//ListenLeaseRespChan 监听 续租情况
func (p *EtcdPlugin) listenLeaseRespChan() {
	for leaseKeepResp := range p.keepAliveChan {
		log.Rlog.Debug("[etcd plugin] %v", leaseKeepResp)
	}
}

// Close 注销服务
func (p *EtcdPlugin) Close() error {
	//撤销租约
	if _, err := p.client.Revoke(context.Background(), p.leaseID); err != nil {
		return err
	}
	return p.client.Close()
}