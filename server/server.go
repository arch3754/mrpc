package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"github.com/arch3754/mrpc/util/cpu"
	"io"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	listener          net.Listener
	Plugins           []Plugin
	TlsConfig         *tls.Config
	connReadIdleTime  time.Duration
	connWriteIdleTime time.Duration
	handlerMap        map[string]*handler
	mu                sync.Mutex
	activeConnMap     map[string]net.Conn
}

func NewServer(readIdleTimeout,writeIdleTimeout time.Duration) *Server {
	return &Server{
		handlerMap: make(map[string]*handler),
		activeConnMap: make(map[string]net.Conn),
		connReadIdleTime: readIdleTimeout,
		connWriteIdleTime: writeIdleTimeout,
	}
}
func (s *Server) AddPlugin(plugin Plugin) {
	s.Plugins = append(s.Plugins, plugin)
}
func (s *Server) Serve(network, address string) error {
	var ln net.Listener
	var err error
	if s.TlsConfig == nil {
		ln, err = net.Listen(network, address)
	} else {
		ln, err = tls.Listen(network, address, s.TlsConfig)
	}
	if err != nil {
		return err
	}
	for _, plug := range s.Plugins {
		if err := plug.ServiceRegister(); err != nil {
			return err
		}
	}
	s.serveListener(ln)
	return nil
}

func (s *Server) serveListener(ln net.Listener) {
	s.listener = ln
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Rlog.Error("accept err:%v", err)
			return
		}
		s.mu.Lock()
		s.activeConnMap[conn.RemoteAddr().String()] = conn
		s.mu.Unlock()
		if s.TlsConfig != nil {
			if tlsConn, ok := conn.(*tls.Conn); ok {
				if err = tlsConn.Handshake(); err != nil {
					log.Rlog.Error("%v TLS handshake error: %v", conn.RemoteAddr(), err)
					return
				}
			}
		}
		s.serveConn(conn)

		s.mu.Lock()
		delete(s.activeConnMap, conn.RemoteAddr().String())
		s.mu.Unlock()
	}
}
func (s *Server) serveConn(conn net.Conn) {
	r := bufio.NewReaderSize(conn, 1024)
	for {
		now := time.Now()

		ctx := util.WithValue(context.Background(), util.ConnPtr, conn)
		req, err := s.readRequest(ctx, r)
		if err != nil {
			if err == io.EOF {
				log.Rlog.Info("client closed the connection: %s", conn.RemoteAddr().String())
			} else {
				log.Rlog.Info("read request[%v] err: %v", conn.RemoteAddr().String(), err)
			}
			protocol.FreeMsg(req)
			return
		}

		_ = conn.SetReadDeadline(now.Add(s.connReadIdleTime))
		ctx = util.WithLocalValue(ctx, util.RequestTime, time.Now().Unix())
		//todo 池化
		go s.serverRequest(ctx, req, conn, now)

	}
}

func (s *Server) serverRequest(ctx *util.Context, req *protocol.Message, conn net.Conn, now time.Time) {
	if req.IsHbs() {
		resp := req
		resp.SetMessageType(protocol.Response)
		if req.Metadata == nil {
			req.Metadata = make(map[string]string)
		}
		resp.Metadata[util.CpuIdle] = fmt.Sprintf("%v", cpu.CpuIdle())
		_ = conn.SetWriteDeadline(now.Add(s.connWriteIdleTime))
		data := resp.Encode()
		_, err := conn.Write(*data)
		if err != nil {
			log.Rlog.Debug("hbs conn %v err:%v", conn.RemoteAddr(), err)
		}
		protocol.FreeMsg(req)
		return
	}
	respMetaData := make(map[string]string)
	ctx = util.WithLocalValue(util.WithLocalValue(ctx, util.RequestMetaData, req.Metadata),
		util.ResponseMetaData, respMetaData)

	cancelFunc := parseServerTimeout(ctx, req)
	if cancelFunc != nil {
		defer cancelFunc()
	}

	resp := s.handleRequest(ctx, req)
	if len(respMetaData) > 0 {
		if resp.Metadata == nil {
			resp.Metadata = make(map[string]string)
		}
		for k, v := range respMetaData {
			resp.Metadata[k] = v
		}
	} else {
		resp.Metadata = respMetaData
	}
	_ = conn.SetWriteDeadline(now.Add(s.connWriteIdleTime))
	data := resp.Encode()
	_, err := conn.Write(*data)
	if err != nil {
		log.Rlog.Error("conn %v err:%v", conn.RemoteAddr(), err)
	}

	protocol.FreeMsg(req)
	protocol.FreeMsg(resp)
}
func parseServerTimeout(ctx *util.Context, req *protocol.Message) context.CancelFunc {
	if req == nil || req.Metadata == nil {
		return nil
	}

	st := req.Metadata[util.ServerTimeout]
	if st == "" {
		return nil
	}

	timeout, err := strconv.ParseInt(st, 10, 64)
	if err != nil {
		return nil
	}

	newCtx, cancel := context.WithTimeout(ctx.Context, time.Duration(timeout)*time.Millisecond)
	ctx.Context = newCtx
	return cancel
}
func (s *Server) readRequest(ctx context.Context, rd io.Reader) (*protocol.Message, error) {
	req := protocol.GetMsg()
	err := req.Decode(rd)
	if err != nil {
		return nil, err
	}
	return req, nil
}
func (s *Server) handleRequest(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := req.Clone()
	resp.SetMessageType(protocol.Response)
	cdc := codec.CodecMap[req.Serialize()]

	handle := s.handlerMap[req.Path]
	md := handle.methodMap[req.Method]

	var arg = argsReplyPools.Get(md.argTy)
	err := cdc.Decode(req.Payload, arg)
	if err != nil {
		log.Rlog.Error("decode err:%v", err)
		argsReplyPools.Put(md.argTy, arg)
		handlerError(resp, err)
		return resp
	}

	reply := argsReplyPools.Get(md.replyTy)
	err = handle.call(ctx, req.Method, reflect.ValueOf(arg), reflect.ValueOf(reply))
	if err != nil {
		argsReplyPools.Put(md.argTy, arg)
		argsReplyPools.Put(md.replyTy, reply)
		handlerError(resp, err)
		return resp
	}

	data, err := cdc.Encode(reply)
	if err != nil {
		argsReplyPools.Put(md.argTy, arg)
		argsReplyPools.Put(md.replyTy, reply)
		handlerError(resp, err)
		return resp
	}

	argsReplyPools.Put(md.argTy, arg)
	argsReplyPools.Put(md.replyTy, reply)
	resp.Payload = data
	return resp
}
func handlerError(resp *protocol.Message, err error) {
	resp.SetStatus(protocol.Error)
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]string)
	}
	resp.Metadata[util.ResponseError] = err.Error()
}