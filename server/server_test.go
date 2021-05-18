package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"go.etcd.io/etcd/clientv3"
	"testing"
	"time"
)

type A struct {
	Num int
}

func (s *A) Add(ctx context.Context, arg *int64, reply *int64) error {
	resMeta := util.GetResponseMetadata(ctx)
	fmt.Printf("resp meta: %+v\n", util.GetRequestMetadata(ctx))
	resMeta["test_resp"] = "22222"
	*reply = *arg + 1
	return nil
}
func TestNewServer1(t *testing.T) {
	plug, err := NewEtcdPlugin(&EtcdConfig{
		RpcServerAddr: "tcp@127.0.0.1:8889",
		EtcdConf:      &clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}},
		Lease: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(time.Minute,time.Minute)
	s.AddPlugin(plug)
	s.Register(new(A))
	t.Logf("start rpc server,listen on 127.0.0.1:8889")
	t.Fatal("111", s.Serve("tcp", "127.0.0.1:8889"))
}
func TestNewServer(t *testing.T) {
	plug, err := NewEtcdPlugin(&EtcdConfig{
		RpcServerAddr: "tcp@127.0.0.1:8888",
		EtcdConf:      &clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(time.Minute,time.Minute)
	s.AddPlugin(plug)
	s.Register(new(A))
	t.Logf("start rpc server,listen on 127.0.0.1:8888")
	t.Fatal("111", s.Serve("tcp", "127.0.0.1:8888"))
}
func TestHandleRequest(t *testing.T) {
	req := protocol.NewMessage()
	req.SetMessageType(protocol.Request)
	req.SetSeq(123)
	req.SetCompress(protocol.Gzip)
	req.SetStatus(protocol.Normal)
	req.SetSerialize(protocol.Json)
	req.SetVersion(1)
	req.SetHbs(false)
	req.Path = "A"
	req.Method = "Add"
	var argv = 1
	data, err := json.Marshal(argv)
	if err != nil {
		t.Fatal(err)
	}
	req.Payload = data
	server := NewServer(time.Minute,time.Minute)
	server.Register(new(A), "")
	res := server.handleRequest(context.Background(), req)

	if res.Payload == nil {
		t.Fatalf("expect reply but got %s", res.Payload)
	}
	var reply int

	codec := codec.CodecMap[res.Serialize()]
	if codec == nil {
		t.Fatalf("can not find codec %c", codec)
	}

	err = codec.Decode(res.Payload, &reply)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	t.Logf("reply:%v", reply)
}