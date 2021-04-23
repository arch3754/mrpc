package server

import (
	"context"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/util"
	"reflect"
)

type handler struct {
	name      string
	typ       reflect.Type
	val       reflect.Value
	methodMap map[string]*method
}
type method struct {
	name    string
	argTy   reflect.Type
	replyTy reflect.Type
	method  reflect.Method
}

func (s *Server) Register(service interface{}, name ...string) {
	if len(name) > 0 {
		hl := newHandler(service)
		for _, v := range name {
			if len(v) != 0 {
				s.handlerMap[v] = hl
			}
		}
		return
	}
	s.handlerMap[reflect.Indirect(reflect.ValueOf(service)).Type().Name()] = newHandler(service)

}

// handlerName methodName arg reply meta
func newHandler(obj interface{}) *handler {
	h := &handler{
		typ: reflect.TypeOf(obj),
		val: reflect.ValueOf(obj),
	}
	h.name = reflect.Indirect(h.val).Type().Name()
	h.methodMap = h.installMethod(h.typ)

	return h
}
func (h *handler) installMethod(typ reflect.Type) map[string]*method {
	methods := make(map[string]*method)

	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		//if len(m.PkgPath) == 0 {
		//	continue
		//}
		if m.Type.NumIn() != 4 {
			log.Rlog.Debug("method[%v] args not enough", m.Name)
			continue
		}
		if !m.Type.In(1).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			log.Rlog.Debug("handler(%v) method(%v) first arg must be context.Context,ignore handler", h.name, m.Name)
			continue
		}
		if m.Type.NumOut() != 1 {
			log.Rlog.Debug("handler(%v) method(%v) has wrong number of outs:%v", h.name, m.Name, m.Type.NumOut())
			continue
		}
		arg := m.Type.In(2)
		if arg.Kind() != reflect.Ptr {
			log.Rlog.Debug("handler(%v) method(%v) arg not ptr(%v)", h.name, m.Name, arg.Kind().String())
			continue
		}
		reply := m.Type.In(3)
		if reply.Kind() != reflect.Ptr {
			log.Rlog.Debug("handler(%v) method(%v) reply not ptr(%v)", h.name, m.Name, reply.Kind().String())
			continue
		}
		methods[m.Name] = &method{name: m.Name, argTy: arg, replyTy: reply, method: m}
		argsReplyPools.Init(arg)
		argsReplyPools.Init(reply)
	}
	return methods
}

func (h *handler) call(ctx context.Context, method string, arg, reply reflect.Value) error {
	defer func() {
		if err := recover(); err != nil {
			log.Rlog.Error("Recovery] panic recovered:\n%s\n%s", err, util.Stack(3))
		}
	}()
	m := h.methodMap[method]
	f := m.method.Func
	v := f.Call([]reflect.Value{h.val, reflect.ValueOf(ctx), arg, reply})
	errInter := v[0].Interface()
	if errInter != nil {
		return errInter.(error)
	}
	return nil
}
