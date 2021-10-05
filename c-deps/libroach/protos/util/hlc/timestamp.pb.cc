// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: util/hlc/timestamp.proto

#include "util/hlc/timestamp.pb.h"

#include <algorithm>

#include <google/protobuf/stubs/common.h>
#include <google/protobuf/stubs/port.h>
#include <google/protobuf/io/coded_stream.h>
#include <google/protobuf/wire_format_lite_inl.h>
#include <google/protobuf/io/zero_copy_stream_impl_lite.h>
// This is a temporary google only hack
#ifdef GOOGLE_PROTOBUF_ENFORCE_UNIQUENESS
#include "third_party/protobuf/version.h"
#endif
// @@protoc_insertion_point(includes)

namespace cockroach {
namespace util {
namespace hlc {
class TimestampDefaultTypeInternal {
 public:
  ::google::protobuf::internal::ExplicitlyConstructed<Timestamp>
      _instance;
} _Timestamp_default_instance_;
}  // namespace hlc
}  // namespace util
}  // namespace cockroach
namespace protobuf_util_2fhlc_2ftimestamp_2eproto {
static void InitDefaultsTimestamp() {
  GOOGLE_PROTOBUF_VERIFY_VERSION;

  {
    void* ptr = &::cockroach::util::hlc::_Timestamp_default_instance_;
    new (ptr) ::cockroach::util::hlc::Timestamp();
    ::google::protobuf::internal::OnShutdownDestroyMessage(ptr);
  }
  ::cockroach::util::hlc::Timestamp::InitAsDefaultInstance();
}

::google::protobuf::internal::SCCInfo<0> scc_info_Timestamp =
    {{ATOMIC_VAR_INIT(::google::protobuf::internal::SCCInfoBase::kUninitialized), 0, InitDefaultsTimestamp}, {}};

void InitDefaults() {
  ::google::protobuf::internal::InitSCC(&scc_info_Timestamp.base);
}

}  // namespace protobuf_util_2fhlc_2ftimestamp_2eproto
namespace cockroach {
namespace util {
namespace hlc {

// ===================================================================

void Timestamp::InitAsDefaultInstance() {
}
#if !defined(_MSC_VER) || _MSC_VER >= 1900
const int Timestamp::kWallTimeFieldNumber;
const int Timestamp::kLogicalFieldNumber;
#endif  // !defined(_MSC_VER) || _MSC_VER >= 1900

Timestamp::Timestamp()
  : ::google::protobuf::MessageLite(), _internal_metadata_(NULL) {
  ::google::protobuf::internal::InitSCC(
      &protobuf_util_2fhlc_2ftimestamp_2eproto::scc_info_Timestamp.base);
  SharedCtor();
  // @@protoc_insertion_point(constructor:cockroach.util.hlc.Timestamp)
}
Timestamp::Timestamp(const Timestamp& from)
  : ::google::protobuf::MessageLite(),
      _internal_metadata_(NULL) {
  _internal_metadata_.MergeFrom(from._internal_metadata_);
  ::memcpy(&wall_time_, &from.wall_time_,
    static_cast<size_t>(reinterpret_cast<char*>(&logical_) -
    reinterpret_cast<char*>(&wall_time_)) + sizeof(logical_));
  // @@protoc_insertion_point(copy_constructor:cockroach.util.hlc.Timestamp)
}

void Timestamp::SharedCtor() {
  ::memset(&wall_time_, 0, static_cast<size_t>(
      reinterpret_cast<char*>(&logical_) -
      reinterpret_cast<char*>(&wall_time_)) + sizeof(logical_));
}

Timestamp::~Timestamp() {
  // @@protoc_insertion_point(destructor:cockroach.util.hlc.Timestamp)
  SharedDtor();
}

void Timestamp::SharedDtor() {
}

void Timestamp::SetCachedSize(int size) const {
  _cached_size_.Set(size);
}
const Timestamp& Timestamp::default_instance() {
  ::google::protobuf::internal::InitSCC(&protobuf_util_2fhlc_2ftimestamp_2eproto::scc_info_Timestamp.base);
  return *internal_default_instance();
}


void Timestamp::Clear() {
// @@protoc_insertion_point(message_clear_start:cockroach.util.hlc.Timestamp)
  ::google::protobuf::uint32 cached_has_bits = 0;
  // Prevent compiler warnings about cached_has_bits being unused
  (void) cached_has_bits;

  ::memset(&wall_time_, 0, static_cast<size_t>(
      reinterpret_cast<char*>(&logical_) -
      reinterpret_cast<char*>(&wall_time_)) + sizeof(logical_));
  _internal_metadata_.Clear();
}

bool Timestamp::MergePartialFromCodedStream(
    ::google::protobuf::io::CodedInputStream* input) {
#define DO_(EXPRESSION) if (!GOOGLE_PREDICT_TRUE(EXPRESSION)) goto failure
  ::google::protobuf::uint32 tag;
  ::google::protobuf::internal::LiteUnknownFieldSetter unknown_fields_setter(
      &_internal_metadata_);
  ::google::protobuf::io::StringOutputStream unknown_fields_output(
      unknown_fields_setter.buffer());
  ::google::protobuf::io::CodedOutputStream unknown_fields_stream(
      &unknown_fields_output, false);
  // @@protoc_insertion_point(parse_start:cockroach.util.hlc.Timestamp)
  for (;;) {
    ::std::pair<::google::protobuf::uint32, bool> p = input->ReadTagWithCutoffNoLastTag(127u);
    tag = p.first;
    if (!p.second) goto handle_unusual;
    switch (::google::protobuf::internal::WireFormatLite::GetTagFieldNumber(tag)) {
      // int64 wall_time = 1;
      case 1: {
        if (static_cast< ::google::protobuf::uint8>(tag) ==
            static_cast< ::google::protobuf::uint8>(8u /* 8 & 0xFF */)) {

          DO_((::google::protobuf::internal::WireFormatLite::ReadPrimitive<
                   ::google::protobuf::int64, ::google::protobuf::internal::WireFormatLite::TYPE_INT64>(
                 input, &wall_time_)));
        } else {
          goto handle_unusual;
        }
        break;
      }

      // int32 logical = 2;
      case 2: {
        if (static_cast< ::google::protobuf::uint8>(tag) ==
            static_cast< ::google::protobuf::uint8>(16u /* 16 & 0xFF */)) {

          DO_((::google::protobuf::internal::WireFormatLite::ReadPrimitive<
                   ::google::protobuf::int32, ::google::protobuf::internal::WireFormatLite::TYPE_INT32>(
                 input, &logical_)));
        } else {
          goto handle_unusual;
        }
        break;
      }

      default: {
      handle_unusual:
        if (tag == 0) {
          goto success;
        }
        DO_(::google::protobuf::internal::WireFormatLite::SkipField(
            input, tag, &unknown_fields_stream));
        break;
      }
    }
  }
success:
  // @@protoc_insertion_point(parse_success:cockroach.util.hlc.Timestamp)
  return true;
failure:
  // @@protoc_insertion_point(parse_failure:cockroach.util.hlc.Timestamp)
  return false;
#undef DO_
}

void Timestamp::SerializeWithCachedSizes(
    ::google::protobuf::io::CodedOutputStream* output) const {
  // @@protoc_insertion_point(serialize_start:cockroach.util.hlc.Timestamp)
  ::google::protobuf::uint32 cached_has_bits = 0;
  (void) cached_has_bits;

  // int64 wall_time = 1;
  if (this->wall_time() != 0) {
    ::google::protobuf::internal::WireFormatLite::WriteInt64(1, this->wall_time(), output);
  }

  // int32 logical = 2;
  if (this->logical() != 0) {
    ::google::protobuf::internal::WireFormatLite::WriteInt32(2, this->logical(), output);
  }

  output->WriteRaw((::google::protobuf::internal::GetProto3PreserveUnknownsDefault()   ? _internal_metadata_.unknown_fields()   : _internal_metadata_.default_instance()).data(),
                   static_cast<int>((::google::protobuf::internal::GetProto3PreserveUnknownsDefault()   ? _internal_metadata_.unknown_fields()   : _internal_metadata_.default_instance()).size()));
  // @@protoc_insertion_point(serialize_end:cockroach.util.hlc.Timestamp)
}

size_t Timestamp::ByteSizeLong() const {
// @@protoc_insertion_point(message_byte_size_start:cockroach.util.hlc.Timestamp)
  size_t total_size = 0;

  total_size += (::google::protobuf::internal::GetProto3PreserveUnknownsDefault()   ? _internal_metadata_.unknown_fields()   : _internal_metadata_.default_instance()).size();

  // int64 wall_time = 1;
  if (this->wall_time() != 0) {
    total_size += 1 +
      ::google::protobuf::internal::WireFormatLite::Int64Size(
        this->wall_time());
  }

  // int32 logical = 2;
  if (this->logical() != 0) {
    total_size += 1 +
      ::google::protobuf::internal::WireFormatLite::Int32Size(
        this->logical());
  }

  int cached_size = ::google::protobuf::internal::ToCachedSize(total_size);
  SetCachedSize(cached_size);
  return total_size;
}

void Timestamp::CheckTypeAndMergeFrom(
    const ::google::protobuf::MessageLite& from) {
  MergeFrom(*::google::protobuf::down_cast<const Timestamp*>(&from));
}

void Timestamp::MergeFrom(const Timestamp& from) {
// @@protoc_insertion_point(class_specific_merge_from_start:cockroach.util.hlc.Timestamp)
  GOOGLE_DCHECK_NE(&from, this);
  _internal_metadata_.MergeFrom(from._internal_metadata_);
  ::google::protobuf::uint32 cached_has_bits = 0;
  (void) cached_has_bits;

  if (from.wall_time() != 0) {
    set_wall_time(from.wall_time());
  }
  if (from.logical() != 0) {
    set_logical(from.logical());
  }
}

void Timestamp::CopyFrom(const Timestamp& from) {
// @@protoc_insertion_point(class_specific_copy_from_start:cockroach.util.hlc.Timestamp)
  if (&from == this) return;
  Clear();
  MergeFrom(from);
}

bool Timestamp::IsInitialized() const {
  return true;
}

void Timestamp::Swap(Timestamp* other) {
  if (other == this) return;
  InternalSwap(other);
}
void Timestamp::InternalSwap(Timestamp* other) {
  using std::swap;
  swap(wall_time_, other->wall_time_);
  swap(logical_, other->logical_);
  _internal_metadata_.Swap(&other->_internal_metadata_);
}

::std::string Timestamp::GetTypeName() const {
  return "cockroach.util.hlc.Timestamp";
}


// @@protoc_insertion_point(namespace_scope)
}  // namespace hlc
}  // namespace util
}  // namespace cockroach
namespace google {
namespace protobuf {
template<> GOOGLE_PROTOBUF_ATTRIBUTE_NOINLINE ::cockroach::util::hlc::Timestamp* Arena::CreateMaybeMessage< ::cockroach::util::hlc::Timestamp >(Arena* arena) {
  return Arena::CreateInternal< ::cockroach::util::hlc::Timestamp >(arena);
}
}  // namespace protobuf
}  // namespace google

// @@protoc_insertion_point(global_scope)
