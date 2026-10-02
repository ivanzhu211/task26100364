package glbcheck

import (
	"encoding/binary"
	"testing"
)

func TestValidMinimalGLB(t *testing.T) {
	bin := make([]byte, 4)
	data := makeGLB(t, baseDoc(4), bin)
	expectOK(t, CheckBytes(data))
}

func TestBadMagic(t *testing.T) {
	data := makeGLB(t, baseDoc(4), make([]byte, 4))
	data[0] = 'X'
	expectCode(t, CheckBytes(data), "container.badMagic")
}

func TestUnsupportedVersion(t *testing.T) {
	data := makeGLB(t, baseDoc(4), make([]byte, 4))
	binary.LittleEndian.PutUint32(data[4:8], 1)
	expectCode(t, CheckBytes(data), "container.unsupportedVersion")
}

func TestTotalLengthMismatch(t *testing.T) {
	data := makeGLB(t, baseDoc(4), make([]byte, 4))
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)+4))
	expectCode(t, CheckBytes(data), "container.lengthMismatch")
}

func TestTooShort(t *testing.T) {
	expectCode(t, CheckBytes([]byte{1, 2, 3}), "container.tooShort")
}

func TestFirstChunkMustBeJSON(t *testing.T) {
	bin := make([]byte, 4)
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeBIN, bin)
	total := glbHeaderLen + len(chunks)
	data := make([]byte, total)
	binary.LittleEndian.PutUint32(data[0:4], glbMagic)
	binary.LittleEndian.PutUint32(data[4:8], glbVersion2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(total))
	copy(data[glbHeaderLen:], chunks)
	expectCode(t, CheckBytes(data), "container.firstChunkNotJSON")
}

func TestExtraChunkRejected(t *testing.T) {
	bin := make([]byte, 4)
	data := makeGLB(t, baseDoc(4), bin)
	// 追加一个额外块，并修正总长度。
	extra := appendChunk(nil, 0x004E4942, make([]byte, 4))
	data = append(data, extra...)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)))
	expectCode(t, CheckBytes(data), "container.tooManyChunks")
}

func TestBinPaddingZeroToThree(t *testing.T) {
	for pad := int64(0); pad <= 3; pad++ {
		// BIN 负载固定 8 字节（4 的倍数），声明长度 8-pad，填充区全零。
		declared := 8 - pad
		data := makeGLB(t, baseDoc(declared), make([]byte, 8))
		expectOK(t, CheckBytes(data))
	}
}

func TestBinExcessPaddingRejected(t *testing.T) {
	// 声明 4，BIN 负载 8（多 4 字节，超过 3 字节填充上限）。
	data := makeGLB(t, baseDoc(4), make([]byte, 8))
	expectCode(t, CheckBytes(data), "buffer.excessPadding")
}

func TestBinNonZeroPaddingRejected(t *testing.T) {
	bin := make([]byte, 8) // 声明 5，填充 3 字节，其中 1 字节非零
	bin[6] = 1
	data := makeGLB(t, baseDoc(5), bin)
	expectCode(t, CheckBytes(data), "buffer.nonZeroPadding")
}

func TestBinShorterThanDeclared(t *testing.T) {
	data := makeGLB(t, baseDoc(8), make([]byte, 4))
	expectCode(t, CheckBytes(data), "buffer.binTooShort")
}

func TestExternalURIRejected(t *testing.T) {
	doc := map[string]any{
		"asset":   map[string]any{"version": "2.0"},
		"buffers": []any{map[string]any{"byteLength": 4, "uri": "data:application/octet-stream;base64,AAAA"}},
	}
	data := makeGLB(t, doc, nil)
	expectCode(t, CheckBytes(data), "buffer.externalURI")
}

func TestTooManyBuffers(t *testing.T) {
	doc := map[string]any{
		"asset": map[string]any{"version": "2.0"},
		"buffers": []any{
			map[string]any{"byteLength": 4},
			map[string]any{"byteLength": 4},
		},
	}
	data := makeGLB(t, doc, make([]byte, 4))
	expectCode(t, CheckBytes(data), "buffer.tooMany")
}

func TestMissingBinChunk(t *testing.T) {
	data := makeGLB(t, baseDoc(4), nil)
	expectCode(t, CheckBytes(data), "buffer.missingBIN")
}

func TestExtensionsRejected(t *testing.T) {
	doc := baseDoc(4)
	doc["extensionsUsed"] = []any{"KHR_draco_mesh_compression"}
	data := makeGLB(t, doc, make([]byte, 4))
	expectCode(t, CheckBytes(data), "gltf.extensionsUnsupported")
}

func TestBadAssetVersion(t *testing.T) {
	doc := map[string]any{"asset": map[string]any{"version": "1.0"}}
	data := makeGLB(t, doc, nil)
	expectCode(t, CheckBytes(data), "gltf.unsupportedVersion")
}
