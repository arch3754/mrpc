package codec

import (
	"fmt"
	"testing"
)

func TestByteCodec(t *testing.T) {
	type a struct {
		aa string
	}
	var i interface{} = &a{aa: "aaa"}
	if data, ok := i.([]byte); ok {
		fmt.Println(1, data)
	}
	if data, ok := i.(*[]byte); ok {
		fmt.Println(2, *data)
	}

	//fmt.Println("%T is not a []byte", i)
}
