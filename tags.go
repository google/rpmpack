// Copyright 2019 Google LLC
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

package rpmpack

// Define only tags which we actually use
const (
	tagHeaderI18NTable = 0x64 // 100
	// Signature tags are obiously overlapping regular header tags..
	sigSHA1        = 0x010d // 269
	sigSHA256      = 0x0111 // 273
	sigSize        = 0x03e8 // 1000
	sigPayloadSize = 0x03ef // 1007

	tagName              = 0x03e8 // 1000
	tagVersion           = 0x03e9 // 1001
	tagRelease           = 0x03ea // 1002
	tagSize              = 0x03f1 // 1009
	tagOS                = 0x03fd // 1021
	tagArch              = 0x03fe // 1022
	tagFileSizes         = 0x0404 // 1028
	tagFileModes         = 0x0406 // 1030
	tagFileMTimes        = 0x040a // 1034
	tagFileDigests       = 0x040b // 1035
	tagFileUserName      = 0x040f // 1039
	tagFileGroupName     = 0x0410 // 1040
	tagFileVerifyFlags   = 0x0415 // 1045
	tagProvides          = 0x0417 // 1047
	tagFileINodes        = 0x0448 // 1096
	tagPrefixes          = 0x044a // 1098
	tagDirindexes        = 0x045c // 1116
	tagBasenames         = 0x045d // 1117
	tagDirnames          = 0x045e // 1118
	tagPayloadFormat     = 0x0464 // 1124
	tagPayloadCompressor = 0x0465 // 1125
	tagPayloadFlags      = 0x0466 // 1126
	tagFileDigestAlgo    = 0x1393 // 5011
)
