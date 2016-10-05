# Copyright (C) 2017 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

glob_all_dirs()

glob(sources
    PATH .
    INCLUDE ".cpp$" ".h$"
    EXCLUDE "_test.cpp$"
)

glob(test_sources
    PATH .
    INCLUDE "_test.cpp$"
)

if(NOT DISABLED_CXX)
    add_library(cc-arena STATIC ${sources})
endif()
