package protocol

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"sync"
)

// Compressor defines a common compression interface.
type Compressor interface {
	Zip([]byte) ([]byte, error)
	Unzip([]byte) ([]byte, error)
}

type RawDataCompressor struct {
}

func (c RawDataCompressor) Zip(data []byte) ([]byte, error) {
	return data, nil
}

func (c RawDataCompressor) Unzip(data []byte) ([]byte, error) {
	return data, nil
}

// GzipCompressor implements gzip compressor.
type GzipCompressor struct {
}

func (c GzipCompressor) Zip(data []byte) ([]byte, error) {
	return Zip(data)
}

func (c GzipCompressor) Unzip(data []byte) ([]byte, error) {
	return Unzip(data)
}

var (
	spWriter sync.Pool
	spReader sync.Pool
	spBuffer sync.Pool
)

func init() {
	spWriter = sync.Pool{New: func() interface{} {
		return new(gzip.Writer)
	}}
	spReader = sync.Pool{New: func() interface{} {
		return new(gzip.Reader)
	}}
	spBuffer = sync.Pool{New: func() interface{} {
		return bytes.NewBuffer(nil)
	}}
}

// Unzip unzips data.
func Unzip(data []byte) ([]byte, error) {
	//buf := bytes.NewBuffer(data)
	//gr, err := gzip.NewReader(buf)
	//if err != nil {
	//	return nil, err
	//}

	buf := spBuffer.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		spBuffer.Put(buf)
	}()

	_, err := buf.Write(data)
	if err != nil {
		return nil, err
	}

	gr := spReader.Get().(*gzip.Reader)
	defer func() {
		spReader.Put(gr)
	}()
	err = gr.Reset(buf)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	data, err = ioutil.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return data, err
}

// Zip zips data.
func Zip(data []byte) ([]byte, error) {
	//buf:=bytes.NewBuffer(data)
	//w:=gzip.NewWriter(buf)
	//w.Flush()
	//w.Close()
	//buf.Bytes()
	buf := spBuffer.Get().(*bytes.Buffer)
	w := spWriter.Get().(*gzip.Writer)
	w.Reset(buf)

	defer func() {
		buf.Reset()
		spBuffer.Put(buf)
		w.Close()
		spWriter.Put(w)
	}()

	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Flush()
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	dec := buf.Bytes()
	return dec, nil
}
