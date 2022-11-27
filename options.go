package plog

import (
	"encoding/base64"

	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Option is a log option.
type Option func(*options)

// Skip returns an [Option] that is used to skip fields based on a predicate.
func Skip(predicate func(protoreflect.FieldDescriptor) bool) Option {
	return func(o *options) {
		o.skip = predicate
	}
}

// Bytes returns an [Option] that is be used to customise how []byte slices
// are printed. The default is base64 encoded string.
func Bytes(transform func([]byte) slog.Value) Option {
	return func(o *options) {
		o.bytes = transform
	}
}

type options struct {
	skip  func(protoreflect.FieldDescriptor) bool
	bytes func([]byte) slog.Value
}

func newOptions(ops []Option) options {
	o := options{
		skip: func(protoreflect.FieldDescriptor) bool { return false },
		bytes: func(b []byte) slog.Value {
			return slog.StringValue(base64.StdEncoding.EncodeToString(b))
		},
	}
	for _, apply := range ops {
		apply(&o)
	}
	return o
}
