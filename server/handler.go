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
	name       string
	requestTy  reflect.Type
	responseTy reflect.Type
	ctxTy      reflect.Type
	method     reflect.Method
}

func (s *Server) Register(service interface{}, name string) {
	if len(name) > 0 {
		s.handlerMap[name] = newHandler(service)
	} else {
		s.handlerMap[reflect.Indirect(reflect.ValueOf(service)).Type().Name()] = newHandler(service)
	}
}

// handlerName methodName request response meta
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
		mtyp := m.Type
		mname := m.Name
		if len(m.PkgPath) == 0 {
			continue
		}
		if mtyp.NumIn() != 4 {
			log.Rlog.Debug("method[%v] args not enough", mname)
		}
		//todo:验证ctx req resp
		ctx := mtyp.In(1)
		request := mtyp.In(2)
		response := mtyp.In(3)
		methods[mname] = &method{name: m.Name, requestTy: request, responseTy: response, ctxTy: ctx}
	}
	return methods
}
func (h *handler) call(ctx context.Context, method string, request, response reflect.Value) error {
	defer func() {
		if err := recover(); err != nil {
			log.Rlog.Error("Recovery] panic recovered:\n%s\n%s", err, util.Stack(3))
		}
	}()
	m := h.methodMap[method]
	f := m.method.Func
	returnv := f.Call([]reflect.Value{h.val, reflect.ValueOf(ctx), request, response})
	errInter := returnv[0].Interface()
	if errInter != nil {
		return errInter.(error)
	}
	return nil
}
