package util

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"runtime"
)

const (
	RequestMetaData  = "request_metadata"
	ResponseMetaData = "response_metadata"
)
const (
	ResponseError = "response_error"
)

func GetRequestMetaData(ctx context.Context) map[string]string {
	value := ctx.Value(RequestMetaData)
	if value != nil {
		return value.(map[string]string)
	}
	return nil
}
func SetRequestMetaData(ctx context.Context, meta map[string]string) context.Context {
	return context.WithValue(ctx, RequestMetaData, meta)
}
func GetResponseMetaData(ctx context.Context) map[string]string {
	value := ctx.Value(ResponseMetaData)
	if value != nil {
		return value.(map[string]string)
	}
	return nil
}
func SetResponseMetaData(ctx context.Context, meta map[string]string) context.Context {
	return context.WithValue(ctx, ResponseMetaData, meta)
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
	reset     = string([]byte{27, 91, 48, 109})
)

func Stack(skip int) []byte {
	buf := new(bytes.Buffer)
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

func source(lines [][]byte, n int) []byte {
	n--
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}
