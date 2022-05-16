// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: protocols/duobft/duobftpb/duobft.proto

package duobftpb

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	commandpb "github.com/ibalajiarun/go-consensus/pkg/command/commandpb"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type InstanceState_Status int32

const (
	InstanceState_None         InstanceState_Status = 0
	InstanceState_Prepared     InstanceState_Status = 1
	InstanceState_PreCommitted InstanceState_Status = 2
	InstanceState_Committed    InstanceState_Status = 3
	InstanceState_Executed     InstanceState_Status = 4
)

var InstanceState_Status_name = map[int32]string{
	0: "None",
	1: "Prepared",
	2: "PreCommitted",
	3: "Committed",
	4: "Executed",
}

var InstanceState_Status_value = map[string]int32{
	"None":         0,
	"Prepared":     1,
	"PreCommitted": 2,
	"Committed":    3,
	"Executed":     4,
}

func (x InstanceState_Status) String() string {
	return proto.EnumName(InstanceState_Status_name, int32(x))
}

func (InstanceState_Status) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{1, 0}
}

type InstanceState_TStatus int32

const (
	InstanceState_TNone      InstanceState_TStatus = 0
	InstanceState_TPrepared  InstanceState_TStatus = 1
	InstanceState_TCommitted InstanceState_TStatus = 3
	InstanceState_TExecuted  InstanceState_TStatus = 4
)

var InstanceState_TStatus_name = map[int32]string{
	0: "TNone",
	1: "TPrepared",
	3: "TCommitted",
	4: "TExecuted",
}

var InstanceState_TStatus_value = map[string]int32{
	"TNone":      0,
	"TPrepared":  1,
	"TCommitted": 3,
	"TExecuted":  4,
}

func (x InstanceState_TStatus) String() string {
	return proto.EnumName(InstanceState_TStatus_name, int32(x))
}

func (InstanceState_TStatus) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{1, 1}
}

type NormalMessage_Type int32

const (
	NormalMessage_Prepare   NormalMessage_Type = 0
	NormalMessage_PreCommit NormalMessage_Type = 1
	NormalMessage_Commit    NormalMessage_Type = 2
)

var NormalMessage_Type_name = map[int32]string{
	0: "Prepare",
	1: "PreCommit",
	2: "Commit",
}

var NormalMessage_Type_value = map[string]int32{
	"Prepare":   0,
	"PreCommit": 1,
	"Commit":    2,
}

func (x NormalMessage_Type) String() string {
	return proto.EnumName(NormalMessage_Type_name, int32(x))
}

func (NormalMessage_Type) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{2, 0}
}

type DuoBFTMessage struct {
	// Types that are valid to be assigned to Type:
	//	*DuoBFTMessage_Normal
	Type isDuoBFTMessage_Type `protobuf_oneof:"type"`
}

func (m *DuoBFTMessage) Reset()         { *m = DuoBFTMessage{} }
func (m *DuoBFTMessage) String() string { return proto.CompactTextString(m) }
func (*DuoBFTMessage) ProtoMessage()    {}
func (*DuoBFTMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{0}
}
func (m *DuoBFTMessage) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *DuoBFTMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_DuoBFTMessage.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *DuoBFTMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DuoBFTMessage.Merge(m, src)
}
func (m *DuoBFTMessage) XXX_Size() int {
	return m.Size()
}
func (m *DuoBFTMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_DuoBFTMessage.DiscardUnknown(m)
}

var xxx_messageInfo_DuoBFTMessage proto.InternalMessageInfo

type isDuoBFTMessage_Type interface {
	isDuoBFTMessage_Type()
	MarshalTo([]byte) (int, error)
	Size() int
}

type DuoBFTMessage_Normal struct {
	Normal *NormalMessage `protobuf:"bytes,1,opt,name=normal,proto3,oneof" json:"normal,omitempty"`
}

func (*DuoBFTMessage_Normal) isDuoBFTMessage_Type() {}

func (m *DuoBFTMessage) GetType() isDuoBFTMessage_Type {
	if m != nil {
		return m.Type
	}
	return nil
}

func (m *DuoBFTMessage) GetNormal() *NormalMessage {
	if x, ok := m.GetType().(*DuoBFTMessage_Normal); ok {
		return x.Normal
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*DuoBFTMessage) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*DuoBFTMessage_Normal)(nil),
	}
}

type InstanceState struct {
	View        View                  `protobuf:"varint,1,opt,name=view,proto3,casttype=View" json:"view,omitempty"`
	Index       Index                 `protobuf:"varint,2,opt,name=index,proto3,casttype=Index" json:"index,omitempty"`
	Status      InstanceState_Status  `protobuf:"varint,3,opt,name=status,proto3,enum=duobftpb.InstanceState_Status" json:"status,omitempty"`
	TStatus     InstanceState_TStatus `protobuf:"varint,4,opt,name=t_status,json=tStatus,proto3,enum=duobftpb.InstanceState_TStatus" json:"t_status,omitempty"`
	Command     *commandpb.Command    `protobuf:"bytes,5,opt,name=command,proto3" json:"command,omitempty"`
	CommandHash []byte                `protobuf:"bytes,6,opt,name=command_hash,json=commandHash,proto3" json:"command_hash,omitempty"`
}

func (m *InstanceState) Reset()         { *m = InstanceState{} }
func (m *InstanceState) String() string { return proto.CompactTextString(m) }
func (*InstanceState) ProtoMessage()    {}
func (*InstanceState) Descriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{1}
}
func (m *InstanceState) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *InstanceState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_InstanceState.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *InstanceState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InstanceState.Merge(m, src)
}
func (m *InstanceState) XXX_Size() int {
	return m.Size()
}
func (m *InstanceState) XXX_DiscardUnknown() {
	xxx_messageInfo_InstanceState.DiscardUnknown(m)
}

var xxx_messageInfo_InstanceState proto.InternalMessageInfo

func (m *InstanceState) GetView() View {
	if m != nil {
		return m.View
	}
	return 0
}

func (m *InstanceState) GetIndex() Index {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *InstanceState) GetStatus() InstanceState_Status {
	if m != nil {
		return m.Status
	}
	return InstanceState_None
}

func (m *InstanceState) GetTStatus() InstanceState_TStatus {
	if m != nil {
		return m.TStatus
	}
	return InstanceState_TNone
}

func (m *InstanceState) GetCommand() *commandpb.Command {
	if m != nil {
		return m.Command
	}
	return nil
}

func (m *InstanceState) GetCommandHash() []byte {
	if m != nil {
		return m.CommandHash
	}
	return nil
}

type NormalMessage struct {
	Type        NormalMessage_Type `protobuf:"varint,1,opt,name=type,proto3,enum=duobftpb.NormalMessage_Type" json:"type,omitempty"`
	View        View               `protobuf:"varint,2,opt,name=view,proto3,casttype=View" json:"view,omitempty"`
	Index       Index              `protobuf:"varint,3,opt,name=index,proto3,casttype=Index" json:"index,omitempty"`
	Command     *commandpb.Command `protobuf:"bytes,4,opt,name=command,proto3" json:"command,omitempty"`
	CommandHash []byte             `protobuf:"bytes,5,opt,name=command_hash,json=commandHash,proto3" json:"command_hash,omitempty"`
}

func (m *NormalMessage) Reset()         { *m = NormalMessage{} }
func (m *NormalMessage) String() string { return proto.CompactTextString(m) }
func (*NormalMessage) ProtoMessage()    {}
func (*NormalMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f5a945891244abaf, []int{2}
}
func (m *NormalMessage) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NormalMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_NormalMessage.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *NormalMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NormalMessage.Merge(m, src)
}
func (m *NormalMessage) XXX_Size() int {
	return m.Size()
}
func (m *NormalMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_NormalMessage.DiscardUnknown(m)
}

var xxx_messageInfo_NormalMessage proto.InternalMessageInfo

func (m *NormalMessage) GetType() NormalMessage_Type {
	if m != nil {
		return m.Type
	}
	return NormalMessage_Prepare
}

func (m *NormalMessage) GetView() View {
	if m != nil {
		return m.View
	}
	return 0
}

func (m *NormalMessage) GetIndex() Index {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *NormalMessage) GetCommand() *commandpb.Command {
	if m != nil {
		return m.Command
	}
	return nil
}

func (m *NormalMessage) GetCommandHash() []byte {
	if m != nil {
		return m.CommandHash
	}
	return nil
}

func init() {
	proto.RegisterEnum("duobftpb.InstanceState_Status", InstanceState_Status_name, InstanceState_Status_value)
	proto.RegisterEnum("duobftpb.InstanceState_TStatus", InstanceState_TStatus_name, InstanceState_TStatus_value)
	proto.RegisterEnum("duobftpb.NormalMessage_Type", NormalMessage_Type_name, NormalMessage_Type_value)
	proto.RegisterType((*DuoBFTMessage)(nil), "duobftpb.DuoBFTMessage")
	proto.RegisterType((*InstanceState)(nil), "duobftpb.InstanceState")
	proto.RegisterType((*NormalMessage)(nil), "duobftpb.NormalMessage")
}

func init() {
	proto.RegisterFile("protocols/duobft/duobftpb/duobft.proto", fileDescriptor_f5a945891244abaf)
}

var fileDescriptor_f5a945891244abaf = []byte{
	// 517 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x53, 0xcf, 0x8a, 0xd3, 0x5e,
	0x18, 0x6d, 0xda, 0x34, 0x6d, 0xbf, 0x36, 0x25, 0x5c, 0x7e, 0xf0, 0x0b, 0xc3, 0x90, 0xd6, 0x0a,
	0xd2, 0x85, 0x26, 0x5a, 0xc1, 0x85, 0xe0, 0x26, 0xfe, 0x61, 0x46, 0x70, 0x1c, 0x32, 0x41, 0xc1,
	0xcd, 0x70, 0x93, 0x5c, 0xd3, 0x68, 0x9b, 0x1b, 0x72, 0x6f, 0x9c, 0x99, 0xa5, 0x6f, 0xe0, 0x63,
	0xf8, 0x28, 0x2e, 0x67, 0xe9, 0x6a, 0x90, 0xf6, 0x2d, 0x66, 0x25, 0xb9, 0xb9, 0xe9, 0x74, 0x90,
	0x0a, 0xae, 0x72, 0x72, 0xbe, 0xf3, 0x9d, 0x7c, 0x9c, 0x43, 0xe0, 0x5e, 0x96, 0x53, 0x4e, 0x43,
	0xba, 0x60, 0x4e, 0x54, 0xd0, 0xe0, 0x23, 0x97, 0x8f, 0x2c, 0x90, 0xc0, 0x16, 0x02, 0xd4, 0xad,
	0xe9, 0xbd, 0xff, 0x62, 0x1a, 0x53, 0x41, 0x3a, 0x25, 0xaa, 0xe6, 0x7b, 0x77, 0xb3, 0xcf, 0xb1,
	0x13, 0xd2, 0xe5, 0x12, 0xa7, 0x51, 0xfd, 0xcc, 0x82, 0x1a, 0x55, 0xa2, 0xc9, 0x6b, 0xd0, 0x5f,
	0x14, 0xd4, 0x7d, 0xe5, 0xbf, 0x21, 0x8c, 0xe1, 0x98, 0xa0, 0x47, 0xa0, 0xa5, 0x34, 0x5f, 0xe2,
	0x85, 0xa9, 0x8c, 0x95, 0x69, 0x7f, 0xf6, 0xbf, 0x5d, 0x7f, 0xc6, 0x3e, 0x12, 0xbc, 0x14, 0x1e,
	0x34, 0x3c, 0x29, 0x74, 0x35, 0x50, 0xf9, 0x45, 0x46, 0x26, 0xdf, 0x5b, 0xa0, 0x1f, 0xa6, 0x8c,
	0xe3, 0x34, 0x24, 0x27, 0x1c, 0x73, 0x82, 0xf6, 0x41, 0xfd, 0x92, 0x90, 0x33, 0x61, 0xa5, 0xba,
	0xdd, 0xeb, 0xab, 0x91, 0xfa, 0x2e, 0x21, 0x67, 0x9e, 0x60, 0xd1, 0x08, 0xda, 0x49, 0x1a, 0x91,
	0x73, 0xb3, 0x29, 0xc6, 0xbd, 0xeb, 0xab, 0x51, 0xfb, 0xb0, 0x24, 0xbc, 0x8a, 0x47, 0x4f, 0x40,
	0x63, 0x1c, 0xf3, 0x82, 0x99, 0xad, 0xb1, 0x32, 0x1d, 0xce, 0xac, 0x9b, 0x5b, 0x6e, 0x7d, 0xc7,
	0x3e, 0x11, 0x2a, 0x4f, 0xaa, 0xd1, 0x53, 0xe8, 0xf2, 0x53, 0xb9, 0xa9, 0x8a, 0xcd, 0xd1, 0xae,
	0x4d, 0x5f, 0xae, 0x76, 0x78, 0x05, 0xd0, 0x7d, 0xe8, 0xc8, 0x84, 0xcc, 0xb6, 0x08, 0x00, 0xd9,
	0x9b, 0xec, 0xec, 0xe7, 0x15, 0xf2, 0x6a, 0x09, 0xba, 0x03, 0x03, 0x09, 0x4f, 0xe7, 0x98, 0xcd,
	0x4d, 0x6d, 0xac, 0x4c, 0x07, 0x5e, 0x5f, 0x72, 0x07, 0x98, 0xcd, 0x27, 0x6f, 0x41, 0x93, 0xd6,
	0x5d, 0x50, 0x8f, 0x68, 0x4a, 0x8c, 0x06, 0x1a, 0x40, 0xf7, 0x38, 0x27, 0x19, 0xce, 0x49, 0x64,
	0x28, 0xc8, 0x80, 0xc1, 0x71, 0x4e, 0x4a, 0xef, 0x84, 0x73, 0x12, 0x19, 0x4d, 0xa4, 0x43, 0xef,
	0xe6, 0xb5, 0x55, 0xca, 0x5f, 0x9e, 0x93, 0xb0, 0x28, 0xdf, 0xd4, 0x89, 0x0b, 0x1d, 0x79, 0x35,
	0xea, 0x41, 0xdb, 0x97, 0x96, 0x3a, 0xf4, 0xfc, 0x2d, 0xcf, 0x21, 0x80, 0xbf, 0x6d, 0x51, 0x8e,
	0xb7, 0x3c, 0xbe, 0x36, 0x41, 0xbf, 0x55, 0x27, 0x7a, 0x58, 0x95, 0x28, 0xaa, 0x1a, 0xce, 0xf6,
	0x77, 0xb4, 0x6e, 0xfb, 0x17, 0x19, 0xf1, 0x84, 0x72, 0x53, 0x6e, 0xf3, 0xef, 0xe5, 0xb6, 0x76,
	0x94, 0xbb, 0x15, 0xb4, 0xfa, 0xef, 0x41, 0xb7, 0xff, 0x0c, 0xda, 0x06, 0xb5, 0xbc, 0x0e, 0xf5,
	0xa1, 0x23, 0x83, 0xa8, 0x62, 0xd9, 0x64, 0x6b, 0x28, 0x08, 0x40, 0x93, 0xb8, 0xe9, 0xbe, 0xff,
	0xb1, 0xb2, 0x94, 0xcb, 0x95, 0xa5, 0xfc, 0x5a, 0x59, 0xca, 0xb7, 0xb5, 0xd5, 0xb8, 0x5c, 0x5b,
	0x8d, 0x9f, 0x6b, 0xab, 0xf1, 0xe1, 0x59, 0x9c, 0xf0, 0x79, 0x11, 0x94, 0xf7, 0x38, 0x49, 0x80,
	0x17, 0xf8, 0x53, 0x82, 0xf3, 0x22, 0x75, 0x62, 0xfa, 0x20, 0xa4, 0x29, 0x23, 0x29, 0x2b, 0x98,
	0xb3, 0xf3, 0x2f, 0x0d, 0x34, 0x31, 0x7a, 0xfc, 0x3b, 0x00, 0x00, 0xff, 0xff, 0x5d, 0x96, 0x11,
	0x02, 0xc9, 0x03, 0x00, 0x00,
}

func (m *DuoBFTMessage) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *DuoBFTMessage) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DuoBFTMessage) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Type != nil {
		{
			size := m.Type.Size()
			i -= size
			if _, err := m.Type.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
		}
	}
	return len(dAtA) - i, nil
}

func (m *DuoBFTMessage_Normal) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DuoBFTMessage_Normal) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Normal != nil {
		{
			size, err := m.Normal.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDuobft(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}
func (m *InstanceState) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *InstanceState) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *InstanceState) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.CommandHash) > 0 {
		i -= len(m.CommandHash)
		copy(dAtA[i:], m.CommandHash)
		i = encodeVarintDuobft(dAtA, i, uint64(len(m.CommandHash)))
		i--
		dAtA[i] = 0x32
	}
	if m.Command != nil {
		{
			size, err := m.Command.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDuobft(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x2a
	}
	if m.TStatus != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.TStatus))
		i--
		dAtA[i] = 0x20
	}
	if m.Status != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.Status))
		i--
		dAtA[i] = 0x18
	}
	if m.Index != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.Index))
		i--
		dAtA[i] = 0x10
	}
	if m.View != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.View))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *NormalMessage) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NormalMessage) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *NormalMessage) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.CommandHash) > 0 {
		i -= len(m.CommandHash)
		copy(dAtA[i:], m.CommandHash)
		i = encodeVarintDuobft(dAtA, i, uint64(len(m.CommandHash)))
		i--
		dAtA[i] = 0x2a
	}
	if m.Command != nil {
		{
			size, err := m.Command.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDuobft(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if m.Index != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.Index))
		i--
		dAtA[i] = 0x18
	}
	if m.View != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.View))
		i--
		dAtA[i] = 0x10
	}
	if m.Type != 0 {
		i = encodeVarintDuobft(dAtA, i, uint64(m.Type))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintDuobft(dAtA []byte, offset int, v uint64) int {
	offset -= sovDuobft(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *DuoBFTMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Type != nil {
		n += m.Type.Size()
	}
	return n
}

func (m *DuoBFTMessage_Normal) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Normal != nil {
		l = m.Normal.Size()
		n += 1 + l + sovDuobft(uint64(l))
	}
	return n
}
func (m *InstanceState) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.View != 0 {
		n += 1 + sovDuobft(uint64(m.View))
	}
	if m.Index != 0 {
		n += 1 + sovDuobft(uint64(m.Index))
	}
	if m.Status != 0 {
		n += 1 + sovDuobft(uint64(m.Status))
	}
	if m.TStatus != 0 {
		n += 1 + sovDuobft(uint64(m.TStatus))
	}
	if m.Command != nil {
		l = m.Command.Size()
		n += 1 + l + sovDuobft(uint64(l))
	}
	l = len(m.CommandHash)
	if l > 0 {
		n += 1 + l + sovDuobft(uint64(l))
	}
	return n
}

func (m *NormalMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Type != 0 {
		n += 1 + sovDuobft(uint64(m.Type))
	}
	if m.View != 0 {
		n += 1 + sovDuobft(uint64(m.View))
	}
	if m.Index != 0 {
		n += 1 + sovDuobft(uint64(m.Index))
	}
	if m.Command != nil {
		l = m.Command.Size()
		n += 1 + l + sovDuobft(uint64(l))
	}
	l = len(m.CommandHash)
	if l > 0 {
		n += 1 + l + sovDuobft(uint64(l))
	}
	return n
}

func sovDuobft(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozDuobft(x uint64) (n int) {
	return sovDuobft(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *DuoBFTMessage) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDuobft
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: DuoBFTMessage: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: DuoBFTMessage: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Normal", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthDuobft
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDuobft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &NormalMessage{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Type = &DuoBFTMessage_Normal{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDuobft(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *InstanceState) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDuobft
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: InstanceState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: InstanceState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field View", wireType)
			}
			m.View = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.View |= View(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Index", wireType)
			}
			m.Index = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Index |= Index(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Status", wireType)
			}
			m.Status = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Status |= InstanceState_Status(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TStatus", wireType)
			}
			m.TStatus = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TStatus |= InstanceState_TStatus(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Command", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthDuobft
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDuobft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Command == nil {
				m.Command = &commandpb.Command{}
			}
			if err := m.Command.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CommandHash", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDuobft
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDuobft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.CommandHash = append(m.CommandHash[:0], dAtA[iNdEx:postIndex]...)
			if m.CommandHash == nil {
				m.CommandHash = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDuobft(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *NormalMessage) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDuobft
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: NormalMessage: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NormalMessage: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= NormalMessage_Type(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field View", wireType)
			}
			m.View = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.View |= View(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Index", wireType)
			}
			m.Index = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Index |= Index(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Command", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthDuobft
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDuobft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Command == nil {
				m.Command = &commandpb.Command{}
			}
			if err := m.Command.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CommandHash", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDuobft
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDuobft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.CommandHash = append(m.CommandHash[:0], dAtA[iNdEx:postIndex]...)
			if m.CommandHash == nil {
				m.CommandHash = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDuobft(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthDuobft
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipDuobft(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowDuobft
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowDuobft
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthDuobft
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupDuobft
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthDuobft
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthDuobft        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowDuobft          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupDuobft = fmt.Errorf("proto: unexpected end of group")
)
