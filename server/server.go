package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/protocol"
	"github.com/arch3754/mrpc/util"
	"io"
	"net"
	"reflect"
	"sync"
	"time"
)

type Server struct {
	listener      net.Listener
	plugins       []plugin
	tlsConfig     *tls.Config
	handlerMap    map[string]*handler
	mu            sync.Mutex
	activeConnMap map[string]net.Conn
}

func NewServer() *Server {
	return &Server{handlerMap: make(map[string]*handler), activeConnMap: make(map[string]net.Conn)}
}
func (s *Server) Serve(network, address string) error {
	var ln net.Listener
	var err error
	if s.tlsConfig == nil {
		ln, err = net.Listen(network, address)
	} else {
		ln, err = tls.Listen(network, address, s.tlsConfig)
	}
	if err != nil {
		return err
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
		ctx := context.WithValue(context.Background(), "conn", conn)
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
		conn.SetReadDeadline(now.Add(30 * time.Second))
		ctx = util.SetRequestTime(context.Background(), time.Now().Unix())
		go func() {
			if req.IsHbs() {
				req.SetMessageType(protocol.Response)
				data := req.Encode()
				conn.Write(*data)
				return
			}
			respMata := make(map[string]string)
			ctx = util.SetResponseMetaData(util.SetRequestMetaData(ctx, req.Metadata), respMata)
			resp := s.handleRequest(ctx, req)
			conn.SetWriteDeadline(now.Add(30 * time.Second))
			data := resp.Encode()
			conn.Write(*data)
			protocol.FreeMsg(req)
			protocol.FreeMsg(resp)
		}()

	}
}
func (s *Server) readRequest(ctx context.Context, rd io.Reader) (*protocol.Message, error) {
	req := protocol.GetMsg()
	err := req.Decode(rd)
	if err != nil {
		return nil, err
	}
	util.SetRequestMetaData(ctx, req.Metadata)
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
