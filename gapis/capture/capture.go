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

package capture

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/data/pack"
	"github.com/google/gapid/core/data/protoconv"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/math/interval"
	"github.com/google/gapid/gapis/atom"
	"github.com/google/gapid/gapis/atom/atom_pb"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/gfxapi"
	"github.com/google/gapid/gapis/memory"
	"github.com/google/gapid/gapis/replay/value"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/pkg/errors"
)

// The list of captures currently imported.
// TODO: This needs to be moved to persistent storage.
var (
	capturesLock sync.RWMutex
	captures     = []id.ID{}
)

type Capture struct {
	Name     string
	Header   *Header
	Atoms    []atom.Atom
	APIs     []gfxapi.API
	Observed interval.U64RangeList
}

func init() {
	protoconv.Register(toProto, fromProto)
}

// New returns a path to a new capture with the given name, header and atoms.
// The new capture is stored in the database.
func New(ctx context.Context, name string, header *Header, atoms []atom.Atom) (*path.Capture, error) {
	c, err := build(ctx, name, header, atoms, nil)
	if err != nil {
		return nil, err
	}

	id, err := database.Store(ctx, c)
	if err != nil {
		return nil, err
	}

	capturesLock.Lock()
	captures = append(captures, id)
	capturesLock.Unlock()

	return &path.Capture{Id: path.NewID(id)}, nil
}

// NewState returns a new, default-initialized State object built for the
// capture held by the context.
func NewState(ctx context.Context) (*gfxapi.State, error) {
	c, err := Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return c.NewState(), nil
}

// NewState returns a new, default-initialized State object built for the
// capture.
func (c *Capture) NewState() *gfxapi.State {
	freeList := memory.InvertMemoryRanges(c.Observed)
	interval.Remove(&freeList, interval.U64Span{Start: 0, End: value.FirstValidAddress})
	return gfxapi.NewStateWithAllocator(
		memory.NewBasicAllocator(freeList),
		c.Header.Abi.MemoryLayout,
	)
}

// Service returns the service.Capture description for this capture.
func (c *Capture) Service(ctx context.Context, p *path.Capture) *service.Capture {
	apis := make([]*path.API, len(c.APIs))
	for i, a := range c.APIs {
		apis[i] = &path.API{Id: path.NewID(id.ID(a.ID()))}
	}
	observations := make([]*service.MemoryRange, len(c.Observed))
	for i, o := range c.Observed {
		observations[i] = &service.MemoryRange{Base: o.First, Size: o.Count}
	}
	return &service.Capture{
		Name:         c.Name,
		Device:       c.Header.Device,
		Abi:          c.Header.Abi,
		NumCommands:  uint64(len(c.Atoms)),
		Apis:         apis,
		Observations: observations,
	}
}

// Captures returns all the captures stored by the database by identifier.
func Captures() []*path.Capture {
	capturesLock.RLock()
	defer capturesLock.RUnlock()
	out := make([]*path.Capture, len(captures))
	for i, c := range captures {
		out[i] = &path.Capture{Id: path.NewID(c)}
	}
	return out
}

// ResolveFromID resolves a single capture with the ID id.
func ResolveFromID(ctx context.Context, id id.ID) (*Capture, error) {
	obj, err := database.Resolve(ctx, id)
	if err != nil {
		return nil, err
	}
	return obj.(*Capture), nil
}

// ResolveFromPath resolves a single capture with the path p.
func ResolveFromPath(ctx context.Context, p *path.Capture) (*Capture, error) {
	return ResolveFromID(ctx, p.Id.ID())
}

// Import imports the capture by name and data, and stores it in the database.
func Import(ctx context.Context, name string, data []byte) (*path.Capture, error) {
	id, err := database.Store(ctx, &Record{
		Name: name,
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	capturesLock.Lock()
	captures = append(captures, id)
	capturesLock.Unlock()

	return &path.Capture{Id: path.NewID(id)}, nil
}

// Export encodes the given capture and associated resources
// and writes it to the supplied io.Writer in the pack file format,
// producing output suitable for use with Import or opening in the trace editor.
func Export(ctx context.Context, p *path.Capture, w io.Writer) error {
	c, err := ResolveFromPath(ctx, p)
	if err != nil {
		return err
	}
	return c.Export(ctx, w)
}

// Export encodes the given capture and associated resources
// and writes it to the supplied io.Writer in the .gfxtrace format,
// producing output suitable for use with Import or opening in the trace editor.
func (c *Capture) Export(ctx context.Context, w io.Writer) error {
	write, err := pack.NewWriter(w)
	if err != nil {
		return err
	}

	if err := write.Marshal(c.Header, 0); err != nil {
		return err
	}

	// IDs seen, so we can avoid encoding the same resource data multiple times.
	seen := map[id.ID]bool{}

	encodeObservation := func(o atom.Observation) error {
		if seen[o.ID] {
			return nil
		}
		data, err := database.Resolve(ctx, o.ID)
		if err != nil {
			return err
		}
		seen[o.ID] = true
		return write.Marshal(&Resource{Id: o.ID[:], Data: data.([]uint8)}, 0)
	}

	for i, a := range c.Atoms {
		if observations := a.Extras().Observations(); observations != nil {
			for _, r := range observations.Reads {
				if err := encodeObservation(r); err != nil {
					return err
				}
			}
			for _, w := range observations.Writes {
				if err := encodeObservation(w); err != nil {
					return err
				}
			}
		}

		parts, err := atom.Disassemble(a)
		if err != nil {
			return err
		}

		group := uint64(i)
		write.Marshal(&atom_pb.CommandStart{}, group)
		for _, part := range parts {
			msg, err := protoconv.ToProto(ctx, part)
			if err != nil {
				return err
			}
			write.Marshal(msg, group)
		}
		write.Marshal(&atom_pb.CommandEnd{}, group)
	}

	return nil
}

func toProto(ctx context.Context, c *Capture) (*Record, error) {
	buf := bytes.Buffer{}
	if err := c.Export(ctx, &buf); err != nil {
		return nil, err
	}
	return &Record{
		Name: c.Name,
		Data: buf.Bytes(),
	}, nil
}

func fromProto(ctx context.Context, r *Record) (*Capture, error) {
	reader, err := pack.NewReader(bytes.NewReader(r.Data))
	if err != nil {
		return nil, err
	}

	atoms := []atom.Atom{}
	parts := map[uint64][]interface{}{}
	idmap := map[id.ID]id.ID{}

	var header *Header
	for {
		msg, groupID, err := reader.Unmarshal()
		if errors.Cause(err) == io.EOF {
			break
		}
		if err != nil {
			return nil, log.Err(ctx, err, "Failed to unmarshal proto")
		}

		switch msg := msg.(type) {
		case *Header:
			header = msg

		case *Resource:
			var msgID id.ID
			copy(msgID[:], msg.Id)
			id, err := database.Store(ctx, msg.Data)
			if err != nil {
				return nil, err
			}
			if _, dup := idmap[msgID]; dup {
				return nil, log.Errf(ctx, nil, "Duplicate resource with ID: %v", msgID)
			}
			idmap[msgID] = id

		case *atom_pb.CommandStart:
			// TODO: Use this for ordering commands by start.

		case *atom_pb.CommandEnd:
			a, err := atom.Assemble(parts[groupID])
			if err != nil {
				return nil, log.Err(ctx, err, "Failed to assemble atom")
			}
			atoms = append(atoms, a)
			delete(parts, groupID)

		default:
			obj, err := protoconv.ToObject(ctx, msg)
			if err != nil {
				return nil, log.Err(ctx, err, "Failed to convert proto to object")
			}
			parts[groupID] = append(parts[groupID], obj)
		}
	}

	if header == nil {
		return nil, log.Err(ctx, nil, "Capture was missing header chunk")
	}

	return build(ctx, r.Name, header, atoms, idmap)
}

// build creates a capture from the name, header and atoms.
// The atoms are inspected for APIs used and observed memory ranges.
func build(ctx context.Context, name string, header *Header, atoms []atom.Atom, idmap map[id.ID]id.ID) (*Capture, error) {
	out := &Capture{
		Name:     name,
		Header:   header,
		Observed: interval.U64RangeList{},
		APIs:     []gfxapi.API{},
	}

	apiSet := map[gfxapi.ID]gfxapi.API{}

	for _, a := range atoms {
		if api := a.API(); api != nil {
			apiID := api.ID()
			if _, found := apiSet[apiID]; !found {
				apiSet[apiID] = api
				out.APIs = append(out.APIs, api)
			}
		}

		observations := a.Extras().Observations()

		if observations != nil {
			for _, rd := range observations.Reads {
				interval.Merge(&out.Observed, rd.Range.Span(), true)
			}
			for _, wr := range observations.Writes {
				interval.Merge(&out.Observed, wr.Range.Span(), true)
			}
		}

		// Replace resource IDs from identifiers generated at capture time to
		// direct database identifiers. This avoids a database link indirection.
		if observations != nil {
			for i, r := range observations.Reads {
				if id, found := idmap[r.ID]; found {
					observations.Reads[i].ID = id
				}
			}
			for i, w := range observations.Writes {
				if id, found := idmap[w.ID]; found {
					observations.Writes[i].ID = id
				}
			}
		}
		out.Atoms = append(out.Atoms, a)
	}

	return out, nil
}
