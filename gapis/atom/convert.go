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

package atom

import "fmt"

// Assemble constructs an Atom from serialized component parts.
func Assemble(parts []interface{}) (Atom, error) {
	var atom Atom
	for _, part := range parts {
		if a, ok := part.(Atom); ok {
			atom = a
			break
		}
	}
	if atom == nil {
		return nil, fmt.Errorf("Atom not found")
	}
	var invoked bool
	for _, part := range parts {
		switch part := part.(type) {
		case Atom:
			invoked = true

		case Observation:
			observations := atom.Extras().GetOrAppendObservations()
			if !invoked {
				observations.Reads = append(observations.Reads, part)
			} else {
				observations.Writes = append(observations.Writes, part)
			}
		case Extra:
			atom.Extras().Add(part)

		default:
			return nil, fmt.Errorf("Unhandled type during conversion %T:%v", part, part)
		}
	}
	return atom, nil
}

// Disassemble returns the component parts of an Atom for encoding.
func Disassemble(a Atom) ([]interface{}, error) {
	var out []interface{}
	extras := a.Extras()
	observations := extras.Observations()
	for _, e := range extras.All() {
		if e != observations {
			out = append(out, e)
		}
	}
	if observations != nil {
		for _, o := range observations.Reads {
			out = append(out, o)
		}
		out = append(out, a)
		for _, o := range observations.Writes {
			out = append(out, o)
		}
	} else {
		out = append(out, a)
	}
	return out, nil
}
