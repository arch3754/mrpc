package client

import (
	"context"
	"github.com/arch3754/mrpc/util"
	"testing"
	"time"
)

func TestEtcdClient(t *testing.T) {
	c, err := NewEtcdClient([]string{"127.0.0.1:2379"}, "", nil)
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < 10000; i++ {
		time.Sleep(time.Millisecond * 100)
		var reply int64
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ctx = util.SetRequestMetadata(ctx, map[string]string{"testreq": "111"})
		caller := c.AsyncCall(ctx, "A", "Add", time.Now().UnixNano(), &reply)
		select {
		case <-caller.Done:

		}
		if caller.Error != nil {
			t.Error(caller.Error)
			cancel()
		}
		cancel()
		t.Log("reply:", reply, "meta:", caller.ResponseMetadata)
	}

	time.Sleep(time.Minute)
}