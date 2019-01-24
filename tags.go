package rpmpack

// Define only tags which we actually use
const (
	tagHeaderI18NTable = 0x64 // 100
	// Signature tags are obiously overlapping regular header tags..
	sigSHA1        = 0x010d // 269
	sigSize        = 0x03e8 // 1000
	sigPayloadSize = 0x03ef // 1007

	tagName              = 0x03e8 // 1000
	tagVersion           = 0x03e9 // 1001
	tagRelease           = 0x03ea // 1002
	tagSize              = 0x03f1 // 1009
	tagDirindexes        = 0x045c // 1116
	tagBasenames         = 0x045d // 1117
	tagDirnames          = 0x045e // 1118
	tagPayloadFormat     = 0x0464 // 1124
	tagPayloadCompressor = 0x0465 // 1125
	tagPayloadFlags      = 0x0466 // 1126
)
