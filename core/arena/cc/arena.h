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

#ifndef CORE_ARENA_H
#define CORE_ARENA_H

#include <stdint.h>

#include <unordered_set>

namespace core {

// Arena is a memory allocator that owns each of the allocations made by it.
// If there are any outstanding allocations when the Arena is destructed then
// these allocations are automatically freed.
struct Arena {
    Arena();
    ~Arena();

    // allocates a contiguous block of memory of at least the requested size and
    // alignment.
    void* allocate(uint32_t size, uint32_t align);

    // reallocates a block of memory previously allocated by this arena.
    // Data held in the previous allocation will be copied to the reallocated
    // address, but data may be trimmed if the new size is smaller than the
    // previous allocation.
    void* reallocate(void* ptr, uint32_t size, uint32_t align);

    // free releases the memory previously allocated by this arena.
    // Once the memory is freed, it must not be used.
    void free(void* ptr);

    // owns returns true if ptr is owned by this arena.
    bool owns(void* ptr);

private:
    std::unordered_set<void*> allocations;
};

}  // namespace core

#endif //  CORE_ARENA_H
