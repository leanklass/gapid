/*
 * Copyright (C) 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#ifndef GAPII_PACK_ENCODER_H
#define GAPII_PACK_ENCODER_H

#include <memory>
#include <unordered_map>

#include <google/protobuf/io/coded_stream.h>
#include <google/protobuf/io/zero_copy_stream_impl_lite.h>

namespace google {
namespace protobuf {
    class Descriptor;
    class Message;
} // namespace protobuf
} // namespace google

namespace core {
    class StreamWriter;
} // namespace core

namespace gapii {

// PackEncoder provides methods for encoding protobuf messages to the provided
// StreamWriter using the pack-stream format.
class PackEncoder {
public:
    typedef std::shared_ptr<PackEncoder> SPtr;

    // message encodes the protobuf message.
    virtual void message(const ::google::protobuf::Message* msg, uint64_t groupId) = 0;

    // acquireGroup returns a new, currently unusued group identifier starting from 1.
    virtual uint64_t acquireGroup() = 0;

    // releaseGroup returns a group identifier previously returned by acquireGroup.
    virtual void releaseGroup(uint64_t) = 0;

    // create returns a PackEncoder::SPtr that writes to output.
    static SPtr create(std::shared_ptr<core::StreamWriter> output);

    // noop returns a PackEncoder::SPtr that does nothing.
    static SPtr noop();

protected:
    ~PackEncoder() = default;
};

} // namespace gapii

#endif // GAPII_PACK_ENCODER_H
