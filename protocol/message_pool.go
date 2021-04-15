package protocol

import "sync"

var msgPool = sync.Pool{
	New: func() interface{} {
		var h = Header([14]byte{})
		return &Message{
			Header: &h,
		}
	},
}

func GetMsg() *Message {
	return msgPool.Get().(*Message)
}

func FreeMsg(msg *Message) {
	if msg != nil {
		msgPool.Put(msg)
	}
}
