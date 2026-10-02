package glbcheck

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestContainerBadMagic(t *testing.T) {
	data := buildGLB(t, mapDoc(), nil)
	data[0] = 'X'
	res := Check(data)
	requireCode(t, res, "CONTAINER_MAGIC")
}

func TestContainerBadVersion(t *testing.T) {
	data := buildGLB(t, mapDoc(), nil)
	binary.LittleEndian.PutUint32(data[4:8], 1)
	res := Check(data)
	requireCode(t, res, "CONTAINER_VERSION")
}

func TestContainerLengthMismatch(t *testing.T) {
	data := buildGLB(t, mapDoc(), nil)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)+4))
	res := Check(data)
	requireCode(t, res, "CONTAINER_LENGTH")
}

func TestContainerTruncated(t *testing.T) {
	res := Check([]byte{0x67, 0x6C, 0x54})
	requireCode(t, res, "CONTAINER_TRUNCATED")
}

func TestContainerChunkNotAligned(t *testing.T) {
	// JSON 块长度非 4 倍数
	jsonBody := []byte(`{"asset":{"version":"2.0"}}`) // 24 字节，截断 1 字节构造非法
	jsonBody = jsonBody[:len(jsonBody)-1]
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeJSON, jsonBody)
	total := headerSize + len(chunks)
	data := make([]byte, 0, total)
	data = binary.LittleEndian.AppendUint32(data, glbMagic)
	data = binary.LittleEndian.AppendUint32(data, glbVersion)
	data = binary.LittleEndian.AppendUint32(data, uint32(total))
	data = append(data, chunks...)
	res := Check(data)
	requireCode(t, res, "CHUNK_ALIGNMENT")
}

func TestContainerBinFirst(t *testing.T) {
	// BIN 块在 JSON 之前：非法
	jsonBody := []byte(`{"asset":{"version":"2.0"}}`)
	for len(jsonBody)%4 != 0 {
		jsonBody = append(jsonBody, 0x20)
	}
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeBIN, make([]byte, 4))
	chunks = appendChunk(chunks, chunkTypeJSON, jsonBody)
	total := headerSize + len(chunks)
	data := make([]byte, 0, total)
	data = binary.LittleEndian.AppendUint32(data, glbMagic)
	data = binary.LittleEndian.AppendUint32(data, glbVersion)
	data = binary.LittleEndian.AppendUint32(data, uint32(total))
	data = append(data, chunks...)
	res := Check(data)
	requireCode(t, res, "CHUNK_ORDER")
}

func TestContainerUnknownChunk(t *testing.T) {
	jsonBody := []byte(`{"asset":{"version":"2.0"}}`)
	for len(jsonBody)%4 != 0 {
		jsonBody = append(jsonBody, 0x20)
	}
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeJSON, jsonBody)
	chunks = appendChunk(chunks, 0x12345678, make([]byte, 4))
	total := headerSize + len(chunks)
	data := make([]byte, 0, total)
	data = binary.LittleEndian.AppendUint32(data, glbMagic)
	data = binary.LittleEndian.AppendUint32(data, glbVersion)
	data = binary.LittleEndian.AppendUint32(data, uint32(total))
	data = append(data, chunks...)
	res := Check(data)
	requireCode(t, res, "CHUNK_TYPE")
}

func TestBinPaddingValid(t *testing.T) {
	// 声明 13 字节，BIN 块 16 字节（3 字节零填充）：合法
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 13}})
	requireOK(t, Check(buildGLB(t, doc, make([]byte, 13))))
}

func TestBinPaddingNonZero(t *testing.T) {
	// 填充字节非零：非法
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 13}})
	data := buildGLB(t, doc, make([]byte, 13))
	data[len(data)-1] = 0xFF
	res := Check(data)
	requireCode(t, res, "BIN_PADDING")
}

func TestBinTooShort(t *testing.T) {
	// BIN 块比声明短：非法（声明 20，实际 16）
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 20}})
	res := Check(buildGLB(t, doc, make([]byte, 16)))
	requireCode(t, res, "BIN_LENGTH")
}

func TestBinExcessPadding(t *testing.T) {
	// BIN 块比声明多 4 字节以上：非法（声明 12，实际 16）
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 12}})
	res := Check(buildGLB(t, doc, make([]byte, 16)))
	requireCode(t, res, "BIN_PADDING")
}

func TestBufferMissingBin(t *testing.T) {
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 16}})
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "BIN_MISSING")
}

func TestBinWithoutBuffer(t *testing.T) {
	res := Check(buildGLB(t, mapDoc(), make([]byte, 4)))
	requireCode(t, res, "BIN_ORPHAN")
}

func TestMultipleBuffers(t *testing.T) {
	doc := mapDoc("buffers", []any{
		map[string]any{"byteLength": 4},
		map[string]any{"byteLength": 4},
	})
	res := Check(buildGLB(t, doc, make([]byte, 4)))
	requireCode(t, res, "BUFFER_COUNT")
}

func TestExternalURIBuffer(t *testing.T) {
	doc := mapDoc("buffers", []any{map[string]any{"byteLength": 4, "uri": "data:application/octet-stream;base64,AAAA"}})
	res := Check(buildGLB(t, doc, make([]byte, 4)))
	requireCode(t, res, "URI_UNSUPPORTED")
}

func TestExternalURIImage(t *testing.T) {
	doc := mapDoc("images", []any{map[string]any{"uri": "textures/albedo.png"}})
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "URI_UNSUPPORTED")
}

func TestExtensionsRejected(t *testing.T) {
	doc := mapDoc(
		"extensionsUsed", []any{"KHR_draco_mesh_compression"},
		"extensionsRequired", []any{"KHR_draco_mesh_compression"},
	)
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "EXTENSION_UNSUPPORTED")
}

func TestInvalidJSON(t *testing.T) {
	jsonBody := []byte(`{not json`)
	for len(jsonBody)%4 != 0 {
		jsonBody = append(jsonBody, 0x20)
	}
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeJSON, jsonBody)
	total := headerSize + len(chunks)
	data := make([]byte, 0, total)
	data = binary.LittleEndian.AppendUint32(data, glbMagic)
	data = binary.LittleEndian.AppendUint32(data, glbVersion)
	data = binary.LittleEndian.AppendUint32(data, uint32(total))
	data = append(data, chunks...)
	res := Check(data)
	requireCode(t, res, "JSON_INVALID")
}

func TestCheckFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ok.glb"
	data := buildGLB(t, mapDoc(), nil)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := CheckFile(path)
	if err != nil {
		t.Fatalf("CheckFile: %v", err)
	}
	requireOK(t, res)
	if _, err := CheckFile(dir + "/missing.glb"); err == nil {
		t.Fatal("期望读取不存在文件时报错")
	}
}
