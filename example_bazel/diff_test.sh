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
  local result="$1"
  local want="$2"
  local got
  got="$(cat "$result" | sed '1,/===marker===/ d')"

  if ! diff <( echo "$want") <(echo "$got"); then
    echo "diff failed or differences found" >&2
    return 1
  fi
  return 0
}

read -r -d '' GOLDEN_STR << EOF
{GOLDEN}
EOF

diff_test "{RESULT}" "${GOLDEN_STR}"

exit $?
