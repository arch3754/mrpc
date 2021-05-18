package client

import (
	"bufio"
	"context"
	"fmt"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/lb"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"net"
	"strconv"
	"sync"
	"time"
)

type client struct {
	Option        *Option
	conn          net.Conn
	seq           uint64
	mu            sync.Mutex
	callerMap     map[uint64]*Caller
	isClose       bool
	serverCpuIdle float64
}
type Caller struct {
	Path             string
	Method           string
	RequestMetadata  map[string]string
	ResponseMetadata map[string]string
	Arg              interface{}
	Reply            interface{}
	Error            error
	Done             chan *Caller
	seq              uint64
}
type Option struct {
	//Retry              int
	Serialize          protocol.Serialize
	ReadTimeout        time.Duration
	ConnectTimeout     time.Duration
	LoadBalance        int
	HbsEnable          bool
	HbsInterval        time.Duration
	HbsTimeout         time.Duration
	Compress           protocol.Compress
	TCPKeepAlivePeriod time.Duration
	Breaker            Breaker
}

var DefaultOption = &Option{
	Serialize:          protocol.MsgPack,
	ReadTimeout:        time.Second * 10,
	ConnectTimeout:     time.Second * 3,
	LoadBalance:        lb.RoundRobin,
	HbsEnable:          true,
	HbsInterval:        5 * time.Second,
	HbsTimeout:         15 * time.Second,
	Compress:           protocol.Gzip,
	TCPKeepAlivePeriod: time.Second * 60,
	Breaker:            DefaultBreaker,
}

func NewClient(option *Option) *client {
	if option == nil {
		option = DefaultOption
	}
	return &client{
		Option:    option,
		mu:        sync.Mutex{},
		callerMap: make(map[uint64]*Caller),
	}
}
func (c *client) Connect(network string, addr string) error {
	conn, err := c.newTCPConn(network, addr)
	if err != nil {
		log.Rlog.Error("%v", err)
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlivePeriod(c.Option.TCPKeepAlivePeriod)
		_ = tcpConn.SetKeepAlive(true)
	}
	_ = conn.SetDeadline(time.Now().Add(c.Option.ReadTimeout))
	c.conn = conn
	go c.read()
	if c.Option.HbsEnable {
		go c.heartbeatTicker()
	}
	return nil
}
func (c *client) GetCpuIdle() float64 {
	return c.serverCpuIdle
}
func (c *client) GetRemoteAddr() string {
	return c.conn.RemoteAddr().String()
}
func (c *client) Close() error {
	c.isClose = true
	return c.conn.Close()
}
func (c *client) newTCPConn(network, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, c.Option.ConnectTimeout)
}
func (c *client) AsyncCall(ctx context.Context, path, method string, arg, reply interface{}) *Caller {
	caller := &Caller{
		Path:   path,
		Method: method,
		Arg:    arg,
		Reply:  reply,
		Done:   make(chan *Caller, 1),
	}

	meta := ctx.Value(util.RequestMetaData)
	if meta != nil {
		caller.RequestMetadata = meta.(map[string]string)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if len(caller.RequestMetadata) == 0 {
			caller.RequestMetadata = make(map[string]string)
		}
		caller.RequestMetadata[util.ServerTimeout] = fmt.Sprintf("%v", time.Until(deadline).Milliseconds())
	}
	if _, ok := ctx.(*util.Context); !ok {
		ctx = util.NewContext(ctx)
	}
	c.call(ctx, caller)
	return caller
}

func (c *client) SyncCall(ctx context.Context, path, method string, arg, reply interface{}) error {
	caller := &Caller{
		Path:   path,
		Method: method,
		Arg:    arg,
		Reply:  reply,
		Done:   make(chan *Caller, 1),
	}
	meta := ctx.Value(util.RequestMetaData)
	if meta != nil {
		caller.RequestMetadata = meta.(map[string]string)
	}
	if deadline, ok := ctx.Deadline(); ok {
		caller.RequestMetadata[util.ServerTimeout] = fmt.Sprintf("%v", time.Until(deadline).Milliseconds())
	}
	if _, ok := ctx.(*util.Context); !ok {
		ctx = util.NewContext(ctx)
	}
	c.call(ctx, caller)
	var err error
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.callerMap, caller.seq)
		c.mu.Unlock()
		caller.Error = ctx.Err()
		caller.Done <- caller

	case call := <-caller.Done:
		err = call.Error
	}
	return err
}
func (c *client) heartbeatTicker() {
	if c.Option.HbsTimeout == 0 {
		c.Option.HbsTimeout = 30 * time.Second
	}
	if c.Option.HbsInterval == 0 {
		c.Option.HbsInterval = 5 * time.Second
	}
	ticker := time.NewTicker(c.Option.HbsInterval)
	for range ticker.C {
		if c.isClose {
			ticker.Stop()
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), c.Option.HbsTimeout)
		var reply int64
		err := c.heartbeat(ctx, &reply)
		if err != nil {
			log.Rlog.Warn("heartbeat %v err:%v", c.conn.RemoteAddr().String(), err)
		}
		cancel()
	}
}
func (c *client) heartbeat(ctx context.Context, reply interface{}) error {

	caller := c.AsyncCall(ctx, "", "", time.Now().UnixNano(), reply)
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.callerMap, caller.seq)
		c.mu.Unlock()
		caller.Error = ctx.Err()
		caller.Done <- caller
		return ctx.Err()
	case call := <-caller.Done:
		if call.Error != nil {
			return call.Error
		}
		if v, ok := call.ResponseMetadata["cpu.idle"]; ok {
			c.serverCpuIdle, _ = strconv.ParseFloat(v, 64)
			log.Rlog.Debug("heartbeat remoteAddr=%v cpu.idle=%v", c.conn.RemoteAddr().String(), c.serverCpuIdle)
		}
	}
	return nil
}
func (c *client) call(ctx context.Context, caller *Caller) {
	_ = c.conn.SetDeadline(time.Now().Add(c.Option.ReadTimeout))
	c.mu.Lock()
	cdc := codec.CodecMap[c.Option.Serialize]
	seq := c.seq
	c.seq++

	c.callerMap[seq] = caller
	c.mu.Unlock()
	caller.seq = seq
	req := protocol.GetMsg()
	req.SetMessageType(protocol.Request)
	req.SetSeq(seq)
	req.SetHbs(len(caller.Path) == 0 || len(caller.Method) == 0)
	req.Path = caller.Path
	req.Method = caller.Method
	req.SetSerialize(c.Option.Serialize)
	req.Metadata = caller.RequestMetadata
	req.SetCompress(c.Option.Compress)
	data, err := cdc.Encode(caller.Arg)
	if err != nil {
		c.mu.Lock()
		delete(c.callerMap, seq)
		c.mu.Unlock()
		caller.Error = err
		caller.Done <- caller
		return
	}
	req.Payload = data
	data = *req.Encode()
	_, err = c.conn.Write(*req.Encode())
	if err != nil {
		c.mu.Lock()
		delete(c.callerMap, seq)
		c.mu.Unlock()
		caller.Error = err
		caller.Done <- caller
		return
	}
}
func (c *client) read() {
	r := bufio.NewReader(c.conn)
	var err error
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.Option.ReadTimeout))
		var resp = protocol.GetMsg()
		resp.SetMessageType(protocol.Response)
		if err = resp.Decode(r); err != nil {
			log.Rlog.Debug("%v", err)
			break
		}
		c.mu.Lock()
		caller := c.callerMap[resp.Seq()]
		delete(c.callerMap, resp.Seq())
		c.mu.Unlock()
		caller.ResponseMetadata = resp.Metadata
		if resp.Status() == protocol.Error {
			caller.Error = fmt.Errorf("%v", resp.Metadata[util.ResponseError])
			caller.Done <- caller
		} else {
			cdc := codec.CodecMap[resp.Serialize()]
			err = cdc.Decode(resp.Payload, caller.Reply)
			if err != nil {
				caller.Error = err
			}
			caller.Done <- caller
		}
		//if c.isClose {
		//	break
		//}
	}
	c.isClose = true
	_ = c.conn.Close()
	for _, call := range c.callerMap {
		call.Error = fmt.Errorf("connection is closing")
		call.Done <- call
	}

}
