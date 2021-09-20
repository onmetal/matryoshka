// Copyright 2021 OnMetal authors
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

package condition

var (
	// DefaultAccessor is an Accessor initialized with the default fields.
	// See NewAccessor for more.
	DefaultAccessor = NewAccessor(AccessorOptions{})

	// Update updates the condition with the given options.
	// See Accessor.Update for more.
	Update = DefaultAccessor.Update

	// MustUpdate updates the condition with the given options.
	// See Accessor.MustUpdate for more.
	MustUpdate = DefaultAccessor.MustUpdate

	// UpdateSlice updates the slice with the given options.
	// See Accessor.UpdateSlice for more.
	UpdateSlice = DefaultAccessor.UpdateSlice

	// MustUpdateSlice updates the slice with the given options.
	// See Accessor.MustUpdateSlice for more.
	MustUpdateSlice = DefaultAccessor.MustUpdateSlice

	// FindSlice finds the target condition in the given slice.
	// See Accessor.FindSlice for more.
	FindSlice = DefaultAccessor.FindSlice

	// MustFindSlice finds the target condition in the given slice.
	// See Accessor.MustFindSlice for more.
	MustFindSlice = DefaultAccessor.MustFindSlice

	// FindSliceStatus finds the condition status in the given slice.
	// See Accessor.FindSliceStatus for more.
	FindSliceStatus = DefaultAccessor.FindSliceStatus

	// MustFindSliceStatus finds the condition status in the given slice.
	// See Accessor.MustFindSliceStatus for more.
	MustFindSliceStatus = DefaultAccessor.MustFindSliceStatus
)
