package server

import (
	"context"
	"encoding/json"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/protocol"
	"testing"
)

type A struct {
	Num int
}

func (s *A) Add(ctx context.Context, arg *int, reply *int) error {
	*reply = *arg + 1

	return nil
}
func TestNewServer(t *testing.T) {
	s := NewServer()
	s.Register(&A{}, "")
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
	server := NewServer()
	server.Register(new(A), "")
	res, err := server.handleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to hand request: %v", err)
	}

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
