package client

import (
	"bufio"
	"context"
	"fmt"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"net"
	"sync"
	"time"
)

type client struct {
	Option    *Option
	conn      net.Conn
	seq       uint64
	mu        sync.Mutex
	callerMap map[uint64]*Caller
	closing   bool
}
type Caller struct {
	Path             string
	Method           string
	RequestMetadata  map[string]string
	ResponseMetadata map[string]string
	Arg              interface{}
	Reply            interface{}
	Error            error
	Done             chan int
}
type Option struct {
	Retry              int
	RpcPath            string
	Serialize          protocol.Serialize
	ReadTimeout        time.Duration
	ConnectTimeout     time.Duration
	HbsEnable          bool
	HbsInterval        time.Duration
	HbsTimeout         int
	Compress           protocol.Compress
	TCPKeepAlivePeriod time.Duration
}

func newClient(option *Option) *client {
	return &client{
		Option:    option,
		mu:        sync.Mutex{},
		callerMap: make(map[uint64]*Caller),
	}
}
func (c *client) Connect(network string, addr string) error {
	conn, err := c.newTCPConn(network, addr)
	if err != nil {
		log.Rlog.Error("client connect %v@%v failed:%v", err)
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlivePeriod(c.Option.TCPKeepAlivePeriod)
		tcpConn.SetKeepAlive(true)
	}
	conn.SetDeadline(time.Now().Add(c.Option.ReadTimeout))
	c.conn = conn
	go c.read()
	return nil
}
func (c *client) newTCPConn(network, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, c.Option.ConnectTimeout)
}
func (c *client) asyncCall(ctx context.Context, path, method string, arg, reply interface{}) (*Caller, error) {
	caller := &Caller{
		Path:            path,
		Method:          method,
		RequestMetadata: util.GetRequestMetaData(ctx),
		Arg:             arg,
		Reply:           reply,
		Done:            make(chan int, 1),
	}
	err := c.call(ctx, caller)
	if err != nil {
		return nil, err
	}
	return caller, err
}
func (c *client) syncCall(ctx context.Context, path, method string, arg, reply interface{}) error {
	caller := &Caller{
		Path:            path,
		Method:          method,
		RequestMetadata: util.GetRequestMetaData(ctx),
		Arg:             arg,
		Reply:           reply,
		Done:            make(chan int, 1),
	}
	err := c.call(ctx, caller)
	if err != nil {
		return err
	}
	<-caller.Done
	if caller.Error != nil {
		return caller.Error
	}
	return nil
}
func (c *client) call(ctx context.Context, caller *Caller) error {

	c.mu.Lock()
	cdc := codec.CodecMap[c.Option.Serialize]
	seq := c.seq
	c.seq++
	c.callerMap[seq] = caller
	c.mu.Unlock()

	req := protocol.GetMsg()
	req.SetMessageType(protocol.Request)
	req.SetSeq(seq)
	if len(caller.Path) == 0 || len(caller.Method) == 0 {
		req.SetHbs(true)
	}
	req.Path = caller.Path
	req.Method = caller.Method
	req.SetSerialize(c.Option.Serialize)
	req.Metadata = caller.RequestMetadata
	req.SetCompress(c.Option.Compress)
	data, err := cdc.Encode(caller.Arg)
	if err != nil {
		return err
	}
	req.Payload = data
	util.SetRequestMetaData(ctx, req.Metadata)
	_, err = c.conn.Write(*req.Encode())
	if err != nil {
		caller.Error = err
		c.mu.Lock()
		delete(c.callerMap, seq)
		caller.Done <- 1
		c.mu.Unlock()
		return err
	}
	return nil
}
func (c *client) read() {
	r := bufio.NewReader(c.conn)
	var err error
	for {
		c.conn.SetReadDeadline(time.Now().Add(c.Option.ReadTimeout))
		var resp = protocol.GetMsg()
		resp.SetMessageType(protocol.Response)
		if err = resp.Decode(r); err != nil {
			break
		}
		c.mu.Lock()
		caller := c.callerMap[resp.Seq()]
		delete(c.callerMap, resp.Seq())
		c.mu.Unlock()
		caller.ResponseMetadata = resp.Metadata
		if resp.Status() == protocol.Error {
			caller.Error = fmt.Errorf("%v", resp.Metadata[util.ResponseError])
			caller.Done <- 1
		} else {
			cdc := codec.CodecMap[resp.Serialize()]
			err = cdc.Decode(resp.Payload, caller.Reply)
			if err != nil {
				caller.Error = err
			}
			caller.Done <- 1
		}

	}
	for _, call := range c.callerMap {
		call.Error = err
		call.Done <- 1
	}
}