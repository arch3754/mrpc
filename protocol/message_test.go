package protocol

import (
	"bytes"
	"testing"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage()
	msg.SetCompress(None)
	msg.SetMessageType(Request)
	msg.SetSeq(11111)
	msg.SetHbs(false)
	msg.SetVersion(1)
	msg.SetSerialize(Json)
	msg.SetStatus(Normal)
	msg.Path = "test"
	msg.Method = "me"
	msg.Payload = []byte(`{"1":"test"}`)
	msg.Metadata = map[string]string{"111": "111"}
	body := msg.Encode()

	m := msg.Clone()
	err := m.Decode(bytes.NewBuffer(*body))
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(m.MessageType(), m.Compress(), m.Seq(), m.IsHbs(), m.Version(), m.Serialize(), m.Status())
	t.Log(m.Path)
	t.Log(m.Method)
	t.Log(m.Metadata)
	t.Log(string(m.Payload))
}
