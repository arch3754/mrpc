package codec

import (
	"encoding/json"
	"github.com/arch3754/mrpc/protocol"
	"github.com/vmihailenco/msgpack/v5"
)

var CodecMap = map[protocol.Serialize]Codec{
	protocol.Json:    new(Json),
	protocol.MsgPack: new(MsgPack),
}

type Codec interface {
	Encode(i interface{}) ([]byte, error)
	Decode(data []byte, i interface{}) error
}
type Json struct {
}

func (*Json) Encode(i interface{}) ([]byte, error) {
	return json.Marshal(i)
}
func (*Json) Decode(data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}

type MsgPack struct {
}

func (*MsgPack) Encode(i interface{}) ([]byte, error) {
	return msgpack.Marshal(i)
}
func (*MsgPack) Decode(data []byte, i interface{}) error {
	return msgpack.Unmarshal(data, i)
}
