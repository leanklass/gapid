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

#include "arena.h"

#include "core/cc/assert.h"

#include <stdlib.h>

namespace core {

Arena::Arena() {}

Arena::~Arena() {
    for (void* ptr : allocations) {
        free(ptr);
    }
    allocations.clear();
}

void* Arena::allocate(uint32_t size, uint32_t align) {
    void* ptr = malloc(size); // TODO: alignment
    allocations.insert(ptr);
    return ptr;
}

void* Arena::reallocate(void* ptr, uint32_t size, uint32_t align) {
    GAPID_ASSERT(this->owns(ptr));
    void* newptr = realloc(ptr, size); // TODO: alignment
    allocations.erase(ptr);
    allocations.insert(newptr);
    return newptr;
}

void Arena::free(void* ptr) {
    GAPID_ASSERT(this->owns(ptr));
    allocations.erase(ptr);
    free(ptr);
}

bool Arena::owns(void* ptr) {
    return allocations.count(ptr) == 1;
}

}  // namespace core

extern "C" {

native_arena* native_arena_create() {
    return reinterpret_cast<native_arena*>(new core::Arena());
}

void native_arena_destroy(native_arena* na) {
    delete reinterpret_cast<core::Arena*>(na);
}

void* native_arena_alloc(native_arena* na, uint32_t size, uint32_t align) {
    return reinterpret_cast<core::Arena*>(na)->allocate(size, align);
}

void* native_arena_realloc(native_arena* na, void* ptr, uint32_t size, uint32_t align) {
    return reinterpret_cast<core::Arena*>(na)->reallocate(ptr, size, align);
}

void native_arena_free(native_arena* na, void* ptr) {
    reinterpret_cast<core::Arena*>(na)->free(ptr);
}

} // extern "C"

