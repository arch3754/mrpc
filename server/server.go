package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"github.com/arch3754/mrpc/codec"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/protocol"
	"io"
	"net"
	"reflect"
	"time"
)

type Server struct {
	listener      net.Listener
	plugins       []plugin
	tlsConfig     *tls.Config
	handlerMap    map[string]*handler
	activeConnMap map[string]net.Conn
	//seq        uint64
}

func NewServer(addr string) *Server {
	return &Server{}
}
func (s *Server) Serve(network, address string) error {
	var ln net.Listener
	var err error
	if s.tlsConfig == nil {
		ln, err = net.Listen(network, address)
	} else {
		ln, err = tls.Listen(network, address, s.tlsConfig)
	}
	//	ln, err := makeListen(s, network, address)
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

		}
		s.serveConn(conn)
	}
}
func (s *Server) serveConn(conn net.Conn) {
	r := bufio.NewReaderSize(conn, 1024)
	for {
		now := time.Now()
		ctx := context.WithValue(context.Background(), "req_ctx", conn)
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

		go func() {
			if req.IsHbs() {
				req.SetMessageType(protocol.Response)
				data := req.Encode()
				conn.Write(*data)
				return
			}
			ctx = context.WithValue(ctx, "req_metadata", req.Metadata)
			resp, err := s.handleRequest(ctx, req)
			if err != nil {
				log.Rlog.Error("handler err:%v", err)
			}
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
	return req, err
}
func (s *Server) handleRequest(ctx context.Context, req *protocol.Message) (*protocol.Message, error) {
	resp := req.Clone()
	resp.SetMessageType(protocol.Response)
	cdc := codec.CodecMap[req.Serialize()]

	handle := s.handlerMap[req.Path]
	md := handle.methodMap[req.Method]
	var arg = reflect.New(md.requestTy)
	err := cdc.Decode(req.Payload, arg.Interface())
	if err != nil {
		//todo 返回给client错误
		return nil, err
	}
	reply := reflect.New(md.responseTy)
	err = handle.call(ctx, req.Method, arg, reply)
	data, err := cdc.Encode(reply.Interface())
	if err != nil {
		//todo 返回给client错误
		return nil, err
	}
	resp.Payload = data
	return resp, err
}
