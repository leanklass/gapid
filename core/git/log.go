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

package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/gapid/core/log"
)

const logPrettyFormat = "--pretty=format:ǁ%Hǀ%an <%ae>ǀ%sǀ%bǀ%cI"

// Log returns the top count ChangeLists at HEAD after skipping skip.
// Merges are ignored.
func (g Git) Log(ctx log.Context, count int) ([]ChangeList, error) {
	return g.LogSkip(ctx, count, 0)
}

// LogSkip returns the top count ChangeLists at HEAD after skipping skip.
// Merges are ignored.
func (g Git) LogSkip(ctx log.Context, count, skip int) ([]ChangeList, error) {
	str, _, err := g.run(ctx, "log", logPrettyFormat,
		"--no-merges",
		fmt.Sprintf("-%d", count),
		fmt.Sprintf("--skip=%d", skip),
		g.wd)
	if err != nil {
		return nil, err
	}
	return parseLog(str)
}

// LogStream calls f for each ChangeList in the log until there are no more CLs,
// or f returns false.
func (g Git) LogStream(ctx log.Context, f func(ChangeList) bool) error {
	for i, c := 0, 25; true; i += c {
		cls, err := g.LogSkip(ctx, c, i)
		if err != nil {
			return err
		}
		if len(cls) == 0 {
			return nil
		}
		for _, cl := range cls {
			if !f(cl) {
				return nil
			}
		}
	}
	return nil
}

// Parent returns the parent ChangeList for cl.
func (g Git) Parent(ctx log.Context, cl ChangeList) (ChangeList, error) {
	str, _, err := g.run(ctx, "log", logPrettyFormat, fmt.Sprintf("%v^", cl.SHA))
	if err != nil {
		return ChangeList{}, err
	}
	cls, err := parseLog(str)
	if err != nil {
		return ChangeList{}, err
	}
	if len(cls) == 0 {
		return ChangeList{}, fmt.Errorf("Unexpected output")
	}
	return cls[0], nil
}

// HeadCL returns the ChangeList at HEAD.
func (g Git) HeadCL(ctx log.Context) (ChangeList, error) {
	cls, err := g.Log(ctx, 1)
	if err != nil {
		return ChangeList{}, err
	}
	return cls[0], nil
}

func parseLog(str string) ([]ChangeList, error) {
	msgs := strings.Split(str, "ǁ")
	cls := make([]ChangeList, 0, len(msgs))
	for _, s := range msgs {
		if parts := strings.Split(s, "ǀ"); len(parts) == 5 {
			cl := ChangeList{
				Author:      strings.TrimSpace(parts[1]),
				Subject:     strings.TrimSpace(parts[2]),
				Description: strings.TrimSpace(parts[3]),
			}
			committed, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[4]))
			if err != nil {
				return nil, err
			}
			cl.Committed = committed
			if err := cl.SHA.Parse(parts[0]); err != nil {
				return nil, err
			}
			cls = append(cls, cl)
		}
	}
	return cls, nil
}
