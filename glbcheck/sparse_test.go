package glbcheck

import (
	"encoding/binary"
	"testing"
)

// sparseDoc 构造含稀疏访问器的文档。
// bin 布局：view0 = 底值数据，view1 = 稀疏索引，view2 = 稀疏值。
func sparseDoc(indexComponentType int, indexByteLength, valuesByteLength int) map[string]any {
	return mapDoc(
		"buffers", []any{map[string]any{"byteLength": 256}},
		"bufferViews", []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 48},
			map[string]any{"buffer": 0, "byteOffset": 48, "byteLength": indexByteLength},
			map[string]any{"buffer": 0, "byteOffset": 64, "byteLength": valuesByteLength},
		},
		"accessors", []any{
			map[string]any{
				"bufferView": 0, "componentType": 5126, "count": 4, "type": "VEC3",
				"sparse": map[string]any{
					"count": 2,
					"indices": map[string]any{
						"bufferView": 1, "byteOffset": 0, "componentType": indexComponentType,
					},
					"values": map[string]any{
						"bufferView": 2, "byteOffset": 0,
					},
				},
			},
		},
	)
}

// sparseBin 构造 BIN：48 字节底值 + 索引区 + 值区，索引按 uint16 写入。
func sparseBin(indices ...uint16) []byte {
	bin := make([]byte, 256)
	for i, v := range indices {
		binary.LittleEndian.PutUint16(bin[48+i*2:], v)
	}
	return bin
}

func TestSparseValid(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	requireOK(t, Check(buildGLB(t, doc, sparseBin(0, 3))))
}

func TestSparseDuplicateIndex(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	res := Check(buildGLB(t, doc, sparseBin(2, 2)))
	requireCode(t, res, "SPARSE_INDEX_ORDER")
}

func TestSparseDescendingIndex(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	res := Check(buildGLB(t, doc, sparseBin(3, 1)))
	requireCode(t, res, "SPARSE_INDEX_ORDER")
}

func TestSparseIndexOutOfRange(t *testing.T) {
	// 访问器 count=4，索引值 4 越界
	doc := sparseDoc(5123, 4, 24)
	res := Check(buildGLB(t, doc, sparseBin(1, 4)))
	requireCode(t, res, "SPARSE_INDEX_RANGE")
}

func TestSparseIndexUint8(t *testing.T) {
	doc := sparseDoc(5121, 2, 24)
	bin := make([]byte, 256)
	bin[48], bin[49] = 0, 2
	requireOK(t, Check(buildGLB(t, doc, bin)))
}

func TestSparseIndexUint32(t *testing.T) {
	doc := sparseDoc(5125, 8, 24)
	bin := make([]byte, 256)
	binary.LittleEndian.PutUint32(bin[48:], 1)
	binary.LittleEndian.PutUint32(bin[52:], 3)
	requireOK(t, Check(buildGLB(t, doc, bin)))
}

func TestSparseIndexBadComponentType(t *testing.T) {
	// 5126 (float32) 不允许作为稀疏索引
	doc := sparseDoc(5126, 8, 24)
	res := Check(buildGLB(t, doc, make([]byte, 256)))
	requireCode(t, res, "SPARSE_INDEX_TYPE")
}

func TestSparseCountExceedsAccessorCount(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	acc := doc["accessors"].([]any)[0].(map[string]any)
	acc["sparse"].(map[string]any)["count"] = 5 // 访问器 count=4
	res := Check(buildGLB(t, doc, sparseBin(0, 1)))
	requireCode(t, res, "SPARSE_COUNT")
}

func TestSparseIndicesViewWithStride(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	views := doc["bufferViews"].([]any)
	views[1].(map[string]any)["byteStride"] = 4
	res := Check(buildGLB(t, doc, sparseBin(0, 1)))
	requireCode(t, res, "SPARSE_STRIDE")
}

func TestSparseValuesViewWithStride(t *testing.T) {
	doc := sparseDoc(5123, 4, 24)
	views := doc["bufferViews"].([]any)
	views[2].(map[string]any)["byteStride"] = 12
	res := Check(buildGLB(t, doc, sparseBin(0, 1)))
	requireCode(t, res, "SPARSE_STRIDE")
}

func TestSparseValuesRangeOverflow(t *testing.T) {
	// VEC3 float32 紧密 12 字节，count=2 需 24 字节，值视图只有 20 字节
	doc := sparseDoc(5123, 4, 20)
	res := Check(buildGLB(t, doc, sparseBin(0, 1)))
	requireCode(t, res, "DATA_RANGE")
}

func TestSparseIndicesMisaligned(t *testing.T) {
	// uint16 索引，相对偏移 1 未按 2 对齐
	doc := sparseDoc(5123, 6, 24)
	acc := doc["accessors"].([]any)[0].(map[string]any)
	acc["sparse"].(map[string]any)["indices"].(map[string]any)["byteOffset"] = 1
	res := Check(buildGLB(t, doc, make([]byte, 256)))
	requireCode(t, res, "OFFSET_ALIGNMENT")
}

func TestSparseMatrixValuesLayout(t *testing.T) {
	// MAT3 uint8 稀疏值：完整步长 12，末元素 11，count=2 需 23 字节。
	// 值视图 23 字节通过；22 字节拒绝。
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 128}},
		"bufferViews", []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36},  // 底值：3 元素 MAT3
			map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": 2},  // 稀疏索引
			map[string]any{"buffer": 0, "byteOffset": 40, "byteLength": 23}, // 稀疏值
		},
		"accessors", []any{
			map[string]any{
				"bufferView": 0, "componentType": 5121, "count": 3, "type": "MAT3",
				"sparse": map[string]any{
					"count": 2,
					"indices": map[string]any{
						"bufferView": 1, "byteOffset": 0, "componentType": 5121,
					},
					"values": map[string]any{
						"bufferView": 2, "byteOffset": 0,
					},
				},
			},
		},
	)
	bin := make([]byte, 128)
	bin[36], bin[37] = 0, 2 // 稀疏索引（uint8，严格递增）
	requireOK(t, Check(buildGLB(t, doc, bin)))

	views := doc["bufferViews"].([]any)
	views[2].(map[string]any)["byteLength"] = 22
	res := Check(buildGLB(t, doc, bin))
	requireCode(t, res, "DATA_RANGE")
}
