package util

import (
	"context"
	"fmt"
	"reflect"
)

type Context struct {
	body map[interface{}]interface{}
	context.Context
}
func SetRequestMetadata(ctx context.Context,meta map[string]string) context.Context{
	return context.WithValue(ctx, RequestMetaData,meta)
}
func GetRequestMetadata(ctx context.Context) map[string]string {
	return ctx.Value(RequestMetaData).(map[string]string)
}
func GetResponseMetadata(ctx context.Context) map[string]string {
	return ctx.Value(ResponseMetaData).(map[string]string)
}
func NewContext(ctx context.Context) *Context {
	return &Context{
		Context: ctx,
		body:    make(map[interface{}]interface{}),
	}
}

func (c *Context) Value(key interface{}) interface{} {
	if c.body == nil {
		c.body = make(map[interface{}]interface{})
	}

	if v, ok := c.body[key]; ok {
		return v
	}
	return c.Context.Value(key)
}

func (c *Context) SetValue(key, val interface{}) {
	if c.body == nil {
		c.body = make(map[interface{}]interface{})
	}
	c.body[key] = val
}

func (c *Context) String() string {
	return fmt.Sprintf("%v.WithValue(%v)", c.Context, c.body)
}

func WithValue(parent context.Context, key, val interface{}) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	tags := make(map[interface{}]interface{})
	tags[key] = val
	return &Context{Context: parent, body: tags}
}

func WithLocalValue(ctx *Context, key, val interface{}) *Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}

	if ctx.body == nil {
		ctx.body = make(map[interface{}]interface{})
	}

	ctx.body[key] = val
	return ctx
}