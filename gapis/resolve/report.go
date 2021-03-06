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

package resolve

import (
	"context"
	"fmt"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/atom"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/replay"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/stringtable"
)

// Report resolves the report for the given path.
func Report(ctx context.Context, p *path.Report) (*service.Report, error) {
	obj, err := database.Build(ctx, &ReportResolvable{p})
	if err != nil {
		return nil, err
	}
	return obj.(*service.Report), nil
}

func (r *ReportResolvable) newReportItem(s log.Severity, c uint64, m *stringtable.Msg) *service.ReportItemRaw {
	var cmd *path.Command
	if c != uint64(atom.NoID) {
		cmd = r.Path.Capture.Command(c)
	}
	return service.WrapReportItem(&service.ReportItem{
		Severity: service.Severity(s),
		Command:  cmd, // TODO: Subcommands
	}, m)
}

// Resolve implements the database.Resolver interface.
func (r *ReportResolvable) Resolve(ctx context.Context) (interface{}, error) {
	ctx = capture.Put(ctx, r.Path.Capture)

	c, err := capture.Resolve(ctx)
	if err != nil {
		return nil, err
	}

	filter, err := buildFilter(ctx, r.Path.Capture, r.Path.Filter)
	if err != nil {
		return nil, err
	}

	builder := service.NewReportBuilder()

	var lastError interface{}
	var currentAtom uint64
	items := []*service.ReportItemRaw{}
	state := c.NewState()
	state.OnError = func(err interface{}) {
		lastError = err
	}
	state.NewMessage = func(s log.Severity, m *stringtable.Msg) uint32 {
		items = append(items, r.newReportItem(s, currentAtom, m))
		return uint32(len(items) - 1)
	}
	state.AddTag = func(i uint32, t *stringtable.Msg) {
		items[i].Tags = append(items[i].Tags, t)
	}

	issues := map[atom.ID][]replay.Issue{}

	if r.Path.Device != nil {
		// Request is for a replay report too.
		intent := replay.Intent{
			Capture: r.Path.Capture,
			Device:  r.Path.Device,
		}

		mgr := replay.GetManager(ctx)

		// Capture can use multiple APIs.
		// Iterate the APIs in use looking for those that support the
		// QueryIssues interface. Call QueryIssues for each of these APIs.
		for _, api := range c.APIs {
			if qi, ok := api.(replay.QueryIssues); ok {
				apiIssues, err := qi.QueryIssues(ctx, intent, mgr)
				if err != nil {
					issue := replay.Issue{
						Atom:     atom.NoID,
						Severity: service.Severity_ErrorLevel,
						Error:    err,
					}
					issues[atom.NoID] = append(issues[atom.NoID], issue)
					continue
				}
				for _, issue := range apiIssues {
					issues[issue.Atom] = append(issues[issue.Atom], issue)
				}
			}
		}
	}

	process := func(i int, a atom.Atom) {
		items, lastError, currentAtom = items[:0], nil, uint64(i)

		defer func() {
			if err := recover(); err != nil {
				items = append(items, r.newReportItem(log.Fatal, uint64(i),
					messages.ErrCritical(fmt.Sprintf("%s", err))))
			}
		}()

		if as := a.Extras().Aborted(); as != nil && as.IsAssert {
			items = append(items, r.newReportItem(log.Fatal, uint64(i),
				messages.ErrTraceAssert(as.Reason)))
		}

		err := a.Mutate(ctx, state, nil /* no builder, just mutate */)

		if len(items) == 0 {
			if err != nil && !atom.IsAbortedError(err) {
				items = append(items, r.newReportItem(log.Error, uint64(i),
					messages.ErrMessage(err)))
			} else if lastError != nil {
				items = append(items, r.newReportItem(log.Error, uint64(i),
					messages.ErrMessage(fmt.Sprintf("%v", lastError))))
			}
		}
	}

	// Gather report items from the state mutator, and collect together all the
	// APIs in use.
	for i, a := range c.Atoms {
		process(i, a)
		if filter(a, state) {
			for _, item := range items {
				item.Tags = append(item.Tags, getAtomNameTag(a))
				builder.Add(ctx, item)
			}
			for _, issue := range issues[atom.ID(i)] {
				item := r.newReportItem(log.Severity(issue.Severity), uint64(issue.Atom),
					messages.ErrReplayDriver(issue.Error.Error()))
				if int(issue.Atom) < len(c.Atoms) {
					item.Tags = append(item.Tags, getAtomNameTag(c.Atoms[issue.Atom]))
				}
				builder.Add(ctx, item)
			}
		}
	}

	return builder.Build(), nil
}

func getAtomNameTag(a atom.Atom) *stringtable.Msg {
	return messages.TagAtomName(a.AtomName())
}
