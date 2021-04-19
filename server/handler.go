package server

import (
	"context"
	"fmt"
	"github.com/arch3754/mrpc/log"
	"github.com/arch3754/mrpc/util"
	"reflect"
	"sync"
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

func (s *Server) Register(service interface{}, name string) {
	if len(name) > 0 {
		s.handlerMap[name] = newHandler(service)
	} else {
		name = reflect.Indirect(reflect.ValueOf(service)).Type().Name()
		s.handlerMap[name] = newHandler(service)
	}
}

// handlerName methodName arg reply meta
func newHandler(obj interface{}) *handler {
	h := &handler{
		typ: reflect.TypeOf(obj),
		val: reflect.ValueOf(obj),
	}
	h.name = reflect.Indirect(h.val).Type().Name()
	h.methodMap = inMethod(h.typ)

	return h
}
func inMethod(typ reflect.Type) map[string]*method {
	methods := make(map[string]*method)

	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		//if len(m.PkgPath) == 0 {
		//	continue
		//}
		if m.Type.NumIn() != 4 {
			log.Rlog.Debug("method[%v] args not enough", m.Name)
		}
		//todo:验证ctx req resp
		arg := m.Type.In(2)
		reply := m.Type.In(3)
		fmt.Println(m.Name, m.Type)
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

var argsReplyPools = &typePools{
	pools: make(map[reflect.Type]*sync.Pool),
	New: func(t reflect.Type) interface{} {
		var argv reflect.Value

		if t.Kind() == reflect.Ptr { // reply must be ptr
			argv = reflect.New(t.Elem())
		} else {
			argv = reflect.New(t)
		}

		return argv.Interface()
	},
}