package log

import "github.com/astaxie/beego/logs"

var Rlog = logs.NewLogger()

func init() {
	Rlog.EnableFuncCallDepth(true)
	Rlog.SetLogFuncCallDepth(2)
}
