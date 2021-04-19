package client

import (
	"context"
	"github.com/arch3754/mrpc/protocol"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	c := newClient(&Option{
		Retry:              3,
		RpcPath:            "A",
		Serialize:          protocol.Json,
		ReadTimeout:        time.Second * 3,
		ConnectTimeout:     time.Second * 3,
		HbsEnable:          false,
		HbsInterval:        0,
		HbsTimeout:         0,
		Compress:           protocol.Gzip,
		TCPKeepAlivePeriod: time.Second * 30,
	})
	err := c.Connect("tcp", "127.0.0.1:8888")
	if err != nil {
		t.Error(err)
		return
	}
	var reply int
	err = c.syncCall(context.Background(), "A", "Add", 1, &reply, nil)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("reply:", reply)
}
