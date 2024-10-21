package machrpc

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ConvertAnyToPb(params []any) ([]*anypb.Any, error) {
	pbparams := make([]*anypb.Any, len(params))
	var err error
	for i, p := range params {
		if p == nil {
			pbparams[i] = nil
			continue
		}
		switch v := p.(type) {
		case *int:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(*v)))
		case int:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(v)))
		case *uint:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(*v)))
		case uint:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(v)))
		case *int8:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(*v)))
		case int8:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(v)))
		case *uint8:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(*v)))
		case uint8:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(v)))
		case *int16:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(*v)))
		case int16:
			pbparams[i], err = anypb.New(wrapperspb.Int32(int32(v)))
		case *uint16:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(*v)))
		case uint16:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(uint32(v)))
		case *int32:
			pbparams[i], err = anypb.New(wrapperspb.Int32(*v))
		case int32:
			pbparams[i], err = anypb.New(wrapperspb.Int32(v))
		case *uint32:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(*v))
		case uint32:
			pbparams[i], err = anypb.New(wrapperspb.UInt32(v))
		case *int64:
			pbparams[i], err = anypb.New(wrapperspb.Int64(*v))
		case int64:
			pbparams[i], err = anypb.New(wrapperspb.Int64(v))
		case *uint64:
			pbparams[i], err = anypb.New(wrapperspb.UInt64(*v))
		case uint64:
			pbparams[i], err = anypb.New(wrapperspb.UInt64(v))
		case *float32:
			pbparams[i], err = anypb.New(wrapperspb.Float(*v))
		case float32:
			pbparams[i], err = anypb.New(wrapperspb.Float(v))
		case *float64:
			pbparams[i], err = anypb.New(wrapperspb.Double(*v))
		case float64:
			pbparams[i], err = anypb.New(wrapperspb.Double(v))
		case *string:
			pbparams[i], err = anypb.New(wrapperspb.String(*v))
		case string:
			pbparams[i], err = anypb.New(wrapperspb.String(v))
		case *[]byte:
			pbparams[i], err = anypb.New(wrapperspb.Bytes(*v))
		case []byte:
			pbparams[i], err = anypb.New(wrapperspb.Bytes(v))
		case *net.IP:
			pbparams[i], err = anypb.New(wrapperspb.String(v.String()))
		case net.IP:
			pbparams[i], err = anypb.New(wrapperspb.String(v.String()))
		case *time.Time:
			pbparams[i], err = anypb.New(timestamppb.New(*v))
		case time.Time:
			pbparams[i], err = anypb.New(timestamppb.New(v))
		default:
			return nil, fmt.Errorf("unsupported params[%d] type %T", i, p)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "convert params[%d]", i)
		}
	}
	return pbparams, nil
}

func ConvertPbToAny(pbvals []*anypb.Any) []any {
	vals := make([]any, len(pbvals))
	for i, pbval := range pbvals {
		var value any
		switch pbval.TypeUrl {
		case "type.googleapis.com/google.protobuf.StringValue":
			var v wrapperspb.StringValue
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.BoolValue":
			var v wrapperspb.BoolValue
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.BytesValue":
			var v wrapperspb.BytesValue
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.DoubleValue":
			var v wrapperspb.DoubleValue
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.FloatValue":
			var v wrapperspb.FloatValue
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.Int32Value":
			var v wrapperspb.Int32Value
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.UInt32Value":
			var v wrapperspb.UInt32Value
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.Int64Value":
			var v wrapperspb.Int64Value
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.UInt64Value":
			var v wrapperspb.UInt64Value
			pbval.UnmarshalTo(&v)
			value = v.Value
		case "type.googleapis.com/google.protobuf.Timestamp":
			var v timestamppb.Timestamp
			pbval.UnmarshalTo(&v)
			value = v.AsTime()
		case "":
			value = nil
		default:
			value = pbval
		}
		vals[i] = value
	}
	return vals
}

func ConvertAnyToPbTuple(params []any) ([]*AppendDatum, error) {
	tuple := make([]*AppendDatum, len(params))
	for i, p := range params {
		if p == nil {
			tuple[i] = &AppendDatum{Value: &AppendDatum_VNull{VNull: true}}
			continue
		}
		switch v := p.(type) {
		case *int:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(*v)}}
		case int:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(v)}}
		case *int8:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(*v)}}
		case int8:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(v)}}
		case *int16:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(*v)}}
		case int16:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: int32(v)}}
		case *int32:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: *v}}
		case int32:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt32{VInt32: v}}
		case *int64:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt64{VInt64: *v}}
		case int64:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VInt64{VInt64: v}}
		case *float32:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VFloat{VFloat: *v}}
		case float32:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VFloat{VFloat: v}}
		case *float64:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VDouble{VDouble: *v}}
		case float64:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VDouble{VDouble: v}}
		case *string:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VString{VString: *v}}
		case string:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VString{VString: v}}
		case *bool:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VBool{VBool: *v}}
		case bool:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VBool{VBool: v}}
		case []byte:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VBytes{VBytes: v}}
		case *net.IP:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VIp{VIp: v.String()}}
		case net.IP:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VIp{VIp: v.String()}}
		case *time.Time:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VTime{VTime: v.UnixNano()}}
		case time.Time:
			tuple[i] = &AppendDatum{Value: &AppendDatum_VTime{VTime: v.UnixNano()}}
		default:
			return nil, fmt.Errorf("unsupported params[%d] type %T", i, p)
		}
	}
	return tuple, nil
}

func ConvertPbTupleToAny(tuple []*AppendDatum) ([]any, error) {
	values := make([]any, len(tuple))
	for i, d := range tuple {
		switch v := d.Value.(type) {
		case *AppendDatum_VInt32:
			values[i] = v.VInt32
		case *AppendDatum_VUint32:
			values[i] = v.VUint32
		case *AppendDatum_VInt64:
			values[i] = v.VInt64
		case *AppendDatum_VUint64:
			values[i] = v.VUint64
		case *AppendDatum_VFloat:
			values[i] = v.VFloat
		case *AppendDatum_VDouble:
			values[i] = v.VDouble
		case *AppendDatum_VString:
			values[i] = v.VString
		case *AppendDatum_VBool:
			values[i] = v.VBool
		case *AppendDatum_VBytes:
			values[i] = v.VBytes
		case *AppendDatum_VIp:
			values[i] = v.VIp
		case *AppendDatum_VTime:
			values[i] = time.Unix(0, v.VTime)
		case *AppendDatum_VNull:
			values[i] = nil
		default:
			return nil, fmt.Errorf("unhandled datum type %T", v)
		}
	}
	return values, nil
}
