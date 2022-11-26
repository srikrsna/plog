package plog

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_ slog.LogValuer = msgValuer{}
	_ slog.LogValuer = enumValuer{}
	_ slog.LogValuer = mapValuer[string, proto.Message]{}
)

var (
	wktStruct      = (&structpb.Struct{}).ProtoReflect().Descriptor().FullName()
	wktFieldMask   = (&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor().FullName()
	wktTimestamp   = (&timestamppb.Timestamp{}).ProtoReflect().Descriptor().FullName()
	wktDuration    = (&durationpb.Duration{}).ProtoReflect().Descriptor().FullName()
	wktAny         = (&anypb.Any{}).ProtoReflect().Descriptor().FullName()
	wktEmpty       = (&emptypb.Empty{}).ProtoReflect().Descriptor().FullName()
	wktBoolValue   = (&wrapperspb.BoolValue{}).ProtoReflect().Descriptor().FullName()
	wktStringValue = (&wrapperspb.StringValue{}).ProtoReflect().Descriptor().FullName()
	wktBytesValue  = (&wrapperspb.BytesValue{}).ProtoReflect().Descriptor().FullName()
	wktInt32Value  = (&wrapperspb.Int32Value{}).ProtoReflect().Descriptor().FullName()
	wktInt64Value  = (&wrapperspb.Int64Value{}).ProtoReflect().Descriptor().FullName()
	wktUInt32Value = (&wrapperspb.UInt32Value{}).ProtoReflect().Descriptor().FullName()
	wktUInt64Value = (&wrapperspb.UInt64Value{}).ProtoReflect().Descriptor().FullName()
	wktFloatValue  = (&wrapperspb.FloatValue{}).ProtoReflect().Descriptor().FullName()
	wktDoubleValue = (&wrapperspb.DoubleValue{}).ProtoReflect().Descriptor().FullName()

	wktSet = map[protoreflect.FullName]bool{
		wktStruct:      true,
		wktFieldMask:   true,
		wktTimestamp:   true,
		wktDuration:    true,
		wktAny:         true,
		wktEmpty:       true,
		wktBoolValue:   true,
		wktStringValue: true,
		wktBytesValue:  true,
		wktInt32Value:  true,
		wktInt64Value:  true,
		wktUInt32Value: true,
		wktUInt64Value: true,
		wktFloatValue:  true,
		wktDoubleValue: true,
	}
)

// Msg returns a [slog.Attr] for any [proto.Message].
func Msg(key string, m proto.Message) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.AnyValue(msgValuer{m.ProtoReflect()}),
	}
}

// Enum returns a [slog.Attr] for any proto enum value.
func Enum(key string, e protoreflect.Enum) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.AnyValue(enumValuer{e}),
	}
}

// Map returns a [slog.Attr] for any protobuf map type
// with a value type of message.
func Map[
	K string | int32 | uint32 | bool,
	V proto.Message,
](key string, m map[K]V) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.AnyValue(mapValuer[K, V]{M: m}),
	}
}

type mapValuer[K string | int32 | int64 | uint32 | uint64 | bool, V proto.Message] struct {
	M map[K]V
}

func (m mapValuer[K, V]) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, len(m.M))
	for k, v := range m.M {
		var key string
		switch kv := any(k).(type) {
		case string:
			key = kv
		case int32:
			key = strconv.FormatInt(int64(kv), 10)
		case int64:
			key = strconv.FormatInt(kv, 10)
		case uint32:
			key = strconv.FormatUint(uint64(kv), 10)
		case uint64:
			key = strconv.FormatUint(kv, 10)
		case bool:
			key = strconv.FormatBool(kv)
		default:
			// This is not possible guaranteed by generic constraint
			// Until go fixes this
		}
		attrs = append(
			attrs,
			slog.Attr{
				Key:   key,
				Value: slog.AnyValue(msgValuer{v.ProtoReflect()}),
			},
		)
	}
	return slog.GroupValue(attrs...)
}

type enumValuer struct {
	protoreflect.Enum
}

func (e enumValuer) LogValue() slog.Value {
	if stringer, ok := e.Enum.(fmt.Stringer); ok {
		return slog.StringValue(stringer.String())
	}
	n := e.Number()
	if ev := e.Descriptor().Values().ByNumber(n); ev != nil {
		return slog.StringValue(string(ev.Name()))
	}
	return slog.StringValue(strconv.Itoa(int(n)))
}

type msgValuer struct {
	protoreflect.Message
}

func (m msgValuer) LogValue() slog.Value {
	if m.Message == nil {
		return slog.GroupValue()
	}
	if wktSet[m.Descriptor().FullName()] {
		return getWKTValue(m.Message)
	}
	fields := m.Descriptor().Fields()
	attrs := make([]slog.Attr, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		// Skip unpopulated
		if !m.Has(fd) {
			continue
		}
		attr := slog.Attr{
			Key:   string(fd.Name()),
			Value: getValueForProtoValue(fd, m.Get(fd)),
		}
		of := fd.ContainingOneof()
		if of != nil && !of.IsSynthetic() {
			attr = slog.Group(string(of.Name()), attr)
		}
		attrs = append(attrs, attr)
	}
	return slog.GroupValue(attrs...)
}

func getValueForProtoValue(
	fd protoreflect.FieldDescriptor,
	v protoreflect.Value,
) slog.Value {
	switch {
	case fd.IsList():
		l := v.List()
		values := make([]slog.Value, 0, l.Len())
		for i := 0; i < l.Len(); i++ {
			values = append(values, getValueByKind(fd, l.Get(i)))
		}
		return slog.AnyValue(values)
	case fd.IsMap():
		m := v.Map()
		attrs := make([]slog.Attr, 0, m.Len())
		m.Range(
			func(mk protoreflect.MapKey, v protoreflect.Value) bool {
				attrs = append(
					attrs,
					slog.Attr{
						Key:   mk.String(),
						Value: getValueByKind(fd.MapValue(), v),
					},
				)
				return true
			},
		)
		return slog.GroupValue(attrs...)
	default:
		return getValueByKind(fd, v)
	}
}

func getValueByKind(
	fd protoreflect.FieldDescriptor,
	v protoreflect.Value,
) slog.Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return slog.BoolValue(v.Bool())
	case protoreflect.DoubleKind, protoreflect.FloatKind:
		return slog.Float64Value(v.Float())
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		return slog.Int64Value(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return slog.Uint64Value(v.Uint())
	case protoreflect.StringKind:
		return slog.StringValue(v.String())
	case protoreflect.BytesKind:
		return slog.StringValue(base64.StdEncoding.EncodeToString(v.Bytes()))
	case protoreflect.EnumKind:
		e := v.Enum()
		if ev := fd.Enum().Values().ByNumber(e); ev != nil {
			return slog.StringValue(string(ev.Name()))
		}
		return slog.StringValue(strconv.Itoa(int(e)))
	case protoreflect.MessageKind:
		return msgValuer{v.Message()}.LogValue()
	default:
		return slog.AnyValue(v.Interface())
	}
}

func getWKTValue(m protoreflect.Message) slog.Value {
	if m.Interface() == nil {
		return slog.AnyValue(nil)
	}
	switch v := m.Interface().(type) {
	case *anypb.Any:
		m, err := anypb.UnmarshalNew(v, proto.UnmarshalOptions{})
		if err != nil {
			return errValue(fmt.Sprintf("failed to unmarshal any: %v", err))
		}
		return msgValuer{m.ProtoReflect()}.LogValue()
	case *fieldmaskpb.FieldMask:
		return slog.StringValue(strings.Join(v.Paths, ","))
	case *structpb.Struct:
		return getStruct(v)
	case *emptypb.Empty:
		return slog.GroupValue()
	case *durationpb.Duration:
		return slog.DurationValue(v.AsDuration())
	case *timestamppb.Timestamp:
		return slog.TimeValue(v.AsTime())
	case *wrapperspb.BoolValue:
		return slog.BoolValue(v.Value)
	case *wrapperspb.BytesValue:
		return slog.StringValue(base64.StdEncoding.EncodeToString(v.Value))
	case *wrapperspb.DoubleValue:
		return slog.Float64Value(v.Value)
	case *wrapperspb.FloatValue:
		return slog.Float64Value(float64(v.Value))
	case *wrapperspb.Int32Value:
		return slog.Int64Value(int64(v.Value))
	case *wrapperspb.Int64Value:
		return slog.Int64Value(v.Value)
	case *wrapperspb.UInt32Value:
		return slog.Uint64Value(uint64(v.Value))
	case *wrapperspb.UInt64Value:
		return slog.Uint64Value(v.Value)
	case *wrapperspb.StringValue:
		return slog.StringValue(v.Value)
	default:
		return errValue("unknow wkt type")
	}
}

func getStruct(s *structpb.Struct) slog.Value {
	attrs := make([]slog.Attr, 0, len(s.Fields))
	for k, v := range s.Fields {
		attrs = append(
			attrs,
			slog.Attr{
				Key:   k,
				Value: getValue(v),
			},
		)
	}
	return slog.GroupValue(attrs...)
}

func getValue(v *structpb.Value) slog.Value {
	switch v := v.Kind.(type) {
	case *structpb.Value_StructValue:
		return getStruct(v.StructValue)
	case *structpb.Value_BoolValue:
		return slog.BoolValue(v.BoolValue)
	case *structpb.Value_NumberValue:
		return slog.Float64Value(v.NumberValue)
	case *structpb.Value_StringValue:
		return slog.StringValue(v.StringValue)
	case *structpb.Value_ListValue:
		values := make([]slog.Value, 0, len(v.ListValue.Values))
		for _, v := range v.ListValue.Values {
			values = append(values, getValue(v))
		}
		return slog.AnyValue(values)
	case *structpb.Value_NullValue:
		return slog.AnyValue(nil)
	default:
		return errValue("unknow struct value")
	}
}

func errValue(err string) slog.Value {
	return slog.StringValue("!error(" + err + ")")
}
