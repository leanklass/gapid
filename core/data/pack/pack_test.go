// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pack_test

import (
	"bytes"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/gapid/core/assert"
	"github.com/google/gapid/core/data/pack"
	"github.com/google/gapid/core/data/protoutil/testprotos"
	"github.com/google/gapid/core/log"
)

func TestReaderWriter(t *testing.T) {
	ctx := log.Testing(t)
	buf := &bytes.Buffer{}

	data := []struct {
		msg proto.Message
		grp uint64
	}{
		{&testprotos.MsgA{F32: 1, U32: 2, S32: 3, Str: "four"}, 0},
		{&testprotos.MsgB{F64: 2, U64: 3, S64: 4, Bool: false}, 1},
		{&testprotos.MsgA{F32: 3, U32: 4, S32: 5, Str: "six"}, 0},
		{&testprotos.MsgB{F64: 4, U64: 5, S64: 6, Bool: true}, 2},
	}

	w, err := pack.NewWriter(buf)
	assert.For(ctx, "NewWriter").ThatError(err).Succeeded()
	for _, data := range data {
		err := w.Marshal(data.msg, data.grp)
		assert.For(ctx, "Marshal(%+v, %v)", data.msg, data.grp).ThatError(err).Succeeded()
	}

	r, err := pack.NewReader(buf)
	assert.For(ctx, "NewReader").ThatError(err).Succeeded()
	for _, data := range data {
		msg, grp, err := r.Unmarshal()
		if !assert.For(ctx, "Unmarshal(%+v, %v)", data.msg, data.grp).ThatError(err).Succeeded() {
			return
		}
		assert.For(ctx, "Unmarshal(%+v, %v).msg", data.msg, data.grp).That(msg).DeepEquals(data.msg)
		assert.For(ctx, "Unmarshal(%+v, %v).grp", data.msg, data.grp).That(grp).Equals(data.grp)
	}
}
