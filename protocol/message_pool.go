package protocol

import "sync"

var msgPool = sync.Pool{
	New: func() interface{} {
		return &Message{
			Header: &Header{},
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
