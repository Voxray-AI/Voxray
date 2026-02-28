// Package serialize provides frame serialization interfaces and implementations.
package serialize

import "voila-go/pkg/frames"

// Serializer converts frames to/from wire format (e.g. JSON envelope or binary protobuf).
type Serializer interface {
	Serialize(f frames.Frame) ([]byte, error)
	Deserialize(data []byte) (frames.Frame, error)
}

// JSONSerializer uses the JSON envelope format (type + data).
type JSONSerializer struct{}

func (JSONSerializer) Serialize(f frames.Frame) ([]byte, error) {
	return Encoder(f)
}

func (JSONSerializer) Deserialize(data []byte) (frames.Frame, error) {
	return Decoder(data)
}

// ProtobufSerializer uses binary protobuf frame format.
// Unserializable frame types are skipped (Serialize returns nil, nil).
type ProtobufSerializer struct{}

func (ProtobufSerializer) Serialize(f frames.Frame) ([]byte, error) {
	return ProtoEncode(f)
}

func (ProtobufSerializer) Deserialize(data []byte) (frames.Frame, error) {
	return ProtoDecode(data)
}
