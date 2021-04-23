package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/arch3754/mrpc/log"
	"io"
)

type Header [14]byte

func (h *Header) Version() byte {
	return h[0]
}
func (h *Header) SetVersion(n byte) {
	h[0] = n
}

type MessageType byte

const (
	Request MessageType = iota
	Response
)

func (h *Header) MessageType() MessageType {
	return MessageType(h[1])
}
func (h *Header) SetMessageType(typ MessageType) {
	h[1] = byte(typ)
}
func (h *Header) IsHbs() bool {
	return h[2] == 1
}
func (h *Header) SetHbs(hbs bool) {
	if hbs {
		h[2] = 1
	} else {
		h[2] = 0
	}
}

type Compress byte

const (
	None Compress = iota
	Gzip
)

var Compressors = map[Compress]Compressor{
	None: &RawDataCompressor{},
	Gzip: &GzipCompressor{},
}

func (h *Header) Compress() Compress {
	return Compress(h[3])
}
func (h *Header) SetCompress(compress Compress) {
	h[3] = byte(compress)
}

type Serialize byte

const (
	Json Serialize = iota
	MsgPack
)

func (h *Header) Serialize() Serialize {
	return Serialize(h[4])
}
func (h *Header) SetSerialize(serialize Serialize) {
	h[4] = byte(serialize)
}

type Status byte

const (
	Normal Status = iota
	Error
)

func (h *Header) Status() Status {
	return Status(h[5])
}
func (h *Header) SetStatus(status Status) {
	h[5] = byte(status)
}

func (h *Header) Seq() uint64 {
	return binary.BigEndian.Uint64(h[6:])
}

func (h *Header) SetSeq(seq uint64) {
	binary.BigEndian.PutUint64(h[6:], seq)
}

type Message struct {
	*Header
	Path     string
	Method   string
	Metadata map[string]string
	Payload  []byte
}

func NewMessage() *Message {
	return &Message{
		Header: &Header{},
	}
}

func (m *Message) Clone() *Message {
	header := *m.Header
	c := GetMsg()
	header.SetCompress(None)
	c.Header = &header
	c.Path = m.Path
	return c
}

func encodeMetadata(m map[string]string) []byte {
	if len(m) == 0 {
		return []byte{}
	}
	var buf bytes.Buffer
	var d = make([]byte, 4)
	for k, v := range m {
		binary.BigEndian.PutUint32(d, uint32(len(k)))
		buf.Write(d)
		buf.Write([]byte(k))
		binary.BigEndian.PutUint32(d, uint32(len(v)))
		buf.Write(d)
		buf.Write([]byte(v))
	}
	return buf.Bytes()
}

func decodeMetadata(l uint32, data []byte) (map[string]string, error) {
	m := make(map[string]string)
	n := uint32(0)
	for n < l {
		sl := binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l-4 {
			return m, errors.New("metadata miss key/value")
		}
		k := string(data[n : n+sl])
		n = n + sl

		sl = binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l {
			return m, errors.New("metadata miss key/value")
		}
		v := string(data[n : n+sl])
		n = n + sl
		m[k] = v
	}
	return m, nil
}
func (m *Message) Encode() *[]byte {
	meta := encodeMetadata(m.Metadata)
	metaSize := len(meta)
	payload := m.Payload
	if m.Compress() != None {
		compress := Compressors[m.Compress()]
		if gzipPayload, err := compress.Zip(payload); err != nil {
			m.SetCompress(None)
		} else {
			payload = gzipPayload
		}
	}
	payloadSize := len(payload)
	path := m.Path
	pathSize := len(path)
	method := m.Method
	methodSize := len(method)
	headerSize := len(m.Header)
	totalSize := 4 + methodSize + 4 + pathSize + 4 + metaSize + 4 + payloadSize
	// header +totolSize+ (pathLen + path) + (metaLen + meta) + (payloadLen + payload)
	//metaStart := headerSize + 4 + (4 + pathSize) + (4 + methodSize)
	//payLoadStart := metaStart + (4 + metaSize)
	//l := 14 + 4 + totalLen
	var data = make([]byte, headerSize+4+totalSize)
	//header
	copy(data, m.Header[:])
	//totalSize
	var n = headerSize

	binary.BigEndian.PutUint32(data[n:n+4], uint32(totalSize))
	//path
	n = n + 4
	binary.BigEndian.PutUint32(data[n:n+4], uint32(pathSize))
	n = n + 4
	copy(data[n:n+pathSize], []byte(path))
	n = n + pathSize
	//method
	binary.BigEndian.PutUint32(data[n:n+4], uint32(methodSize))
	n = n + 4
	copy(data[n:n+methodSize], []byte(method))
	n = n + methodSize
	//meta
	binary.BigEndian.PutUint32(data[n:n+4], uint32(metaSize))
	n = n + 4
	copy(data[n:n+metaSize], meta)
	n = n + metaSize
	//payload
	binary.BigEndian.PutUint32(data[n:n+4], uint32(payloadSize))
	n = n + 4
	copy(data[n:], payload)
	return &data
}
func (m *Message) Decode(r io.Reader) error {
	_, err := io.ReadFull(r, m.Header[:])
	if err != nil {
		return err
	}
	var totolSizeBuf = make([]byte, 4)
	_, err = io.ReadFull(r, totolSizeBuf)
	if err != nil {
		return err
	}
	totalSize := binary.BigEndian.Uint32(totolSizeBuf)
	var data = make([]byte, totalSize)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return err
	}
	var n uint32
	pathSize := binary.BigEndian.Uint32(data[n : n+4])
	n = n + 4
	m.Path = string(data[n : n+pathSize])
	n = n + pathSize
	methodSize := binary.BigEndian.Uint32(data[n : n+4])
	n = n + 4
	m.Method = string(data[n : n+methodSize])
	n = n + methodSize
	metaSize := binary.BigEndian.Uint32(data[n : n+4])
	n = n + 4
	if metaSize > 0 {
		m.Metadata, err = decodeMetadata(metaSize, data[n:n+metaSize])
		if err != nil {
			log.Rlog.Debug("decodeMetadata failed:%v", err)
		}
		n = n + metaSize
	}

	//payloadSize
	//payloadSize := binary.BigEndian.Uint32(data[n : n+4])
	m.Payload = data[n+4:]
	if m.Compress() != None {
		compress := Compressors[m.Compress()]
		m.Payload, err = compress.Unzip(m.Payload)
		if err != nil {
			return err
		}
	}
	return err
}
