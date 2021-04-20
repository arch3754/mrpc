package log

import "github.com/astaxie/beego/logs"

var Rlog = logs.NewLogger(0)

func init() {
	Rlog.SetLevel(logs.LevelDebug)
	Rlog.EnableFuncCallDepth(true)
	Rlog.SetLogFuncCallDepth(2)
}
