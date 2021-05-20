package client

import (
	"context"
	"github.com/arch3754/mrpc/util"
	"testing"
	"time"
)

func BenchmarkClient(b *testing.B) {
	c, err := NewEtcdClient([]string{"127.0.0.1:2379"}, "", DefaultOption)
	if err != nil {
		b.Error(err)
		return
	}
	for i := 0; i < b.N; i++ {
		var reply int64
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ctx = util.SetRequestMetadata(ctx, map[string]string{"testreq": "111"})
		caller := c.AsyncCall(ctx, "A", "Add", time.Now().UnixNano(), &reply)
		select {
		case <-caller.Done:

		}
		if caller.Error != nil {
			b.Error(err)
			cancel()
			return
		}
		cancel()
		//b.Log("reply:", reply, "meta:", caller.ResponseMetadata)
	}
}
func TestEtcdClient(t *testing.T) {
	c, err := NewEtcdClient([]string{"127.0.0.1:2379"}, "", DefaultOption)
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < 1; i++ {
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
	time.Sleep(time.Second * 30)
	var reply int64
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ctx = util.SetRequestMetadata(ctx, map[string]string{"testreq": "222"})
	err = c.Call(ctx, "A", "Add", time.Now().UnixNano(), &reply)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("reply:", reply)
	}
	cancel()
}