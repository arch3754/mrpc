package client

import (
	"context"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"testing"
	"time"
)

func TestEtcdClient(t *testing.T) {
	c, err := NewEtcdClient([]string{"127.0.0.1:2379"}, "", &Option{
		Retry: 3,
		//RpcPath:            "A",
		Serialize:          protocol.Json,
		ReadTimeout:        time.Second * 3,
		ConnectTimeout:     time.Second * 3,
		HbsEnable:          true,
		HbsInterval:        1 * time.Second,
		HbsTimeout:         10 * time.Second,
		Compress:           protocol.Gzip,
		TCPKeepAlivePeriod: time.Second * 300,
	})
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < 10; i++ {
		var reply int64
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ctx = util.SetRequestMetadata(ctx, map[string]string{"testreq": "111"})
		caller := c.AsyncCall(ctx, "A", "Add", time.Now().UnixNano(), &reply)
		select {
		case <-caller.Done:

		}
		if caller.Error != nil {
			t.Error(err)
			cancel()
			return
		}
		cancel()
		t.Log("reply:", reply, "meta:", caller.ResponseMetadata)
	}

	time.Sleep(time.Minute)
}
