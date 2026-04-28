package grpcserver

// jsonCodec is a gRPC codec that serialises messages as JSON.
// Registering it lets us pass plain Go structs (our proto types) without protoc.

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const codecName = "ohe-json"

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Name() string   { return codecName }
func (jsonCodec) String() string { return codecName }

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
