package main

import (
	"context"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/server"
	"github.com/arch3754/mrpc/util"
	"go.etcd.io/etcd/clientv3"
	"time"
)

type A struct {
	Num int
}

func (s *A) Add(ctx context.Context, arg *int64, reply *int64) error {
	resMeta := util.GetResponseMetadata(ctx)
	//log.Rlog.Debug("resp meta: %+v\n", util.GetRequestMetadata(ctx))
	resMeta["test_resp"] = "22222"
	*reply = *arg + 1

	return nil
}
func main() {
	plug, err := server.NewEtcdPlugin(&server.EtcdConfig{
		RpcServerAddr: "tcp@127.0.0.1:8889",
		EtcdConf:      &clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}},
		Lease:         30,
	})
	if err != nil {
		log.Rlog.Error("%v", err)
		return
	}
	s := server.NewServer(time.Second*10, time.Second*10)
	s.AddPlugin(plug)
	s.Register(new(A))
	log.Rlog.Info("start rpc server,listen on 127.0.0.1:8889")
	err = s.Serve("tcp", "127.0.0.1:8889")
	if err != nil {
		log.Rlog.Error("%v", err)
	}
}