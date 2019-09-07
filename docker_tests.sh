#!/bin/bash
#
# Copyright 2019 Google LLC
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

set -o nounset

diff_test () {
  local cmd="$1"
  local want="$2"
  local got
  got="$(set -o pipefail; "$cmd" | sed '1,/===marker===/ d')"
  if [[ "$?" -ne 0 ]]; then
    echo "${cmd}: run failed" >&2
    return 1
  fi
 
  if ! diff "$want" <(echo "$got"); then
    echo "${cmd}: diff failed or differences found" >&2
    return 1
  fi
  return 0
}

diff_test centos_V testdata/golden_V.txt
V_result="$?"
diff_test centos_ls testdata/golden_ls.txt
ls_result="$?"
diff_test centos_preinst testdata/golden_preinst.txt
preinst_result="$?"

if [[ "$V_result" -ne 0 || "$ls_result" -ne 0 || "$preinst_result" -ne 0 ]]; then
  exit 1
fi
