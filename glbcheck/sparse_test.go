package glbcheck

import "testing"

// sparseFixture 构造一个合法的 sparse 文档与 BIN 负载：
// 访问器 count=10 SCALAR float32，sparse count=3，uint8 索引 [1,5,9]，3 个 float32 值。
// 返回文档、BIN 负载以及索引数据在 BIN 中的绝对偏移，便于用例改写索引。
func sparseFixture() (doc map[string]any, bin []byte, idxAbs int) {
	bin = make([]byte, 56)
	idxAbs = 40
	bin[idxAbs+0] = 1
	bin[idxAbs+1] = 5
	bin[idxAbs+2] = 9
	doc = baseDoc(56)
	withViews(doc,
		view(0, 40, nil),  // 0: 主数据
		view(40, 4, nil),  // 1: sparse indices（3 字节 + 1 字节对齐填充）
		view(44, 12, nil), // 2: sparse values
	)
	withAccessors(doc, map[string]any{
		"bufferView": 0, "componentType": 5126, "count": 10, "type": "SCALAR",
		"sparse": map[string]any{
			"count":   3,
			"indices": map[string]any{"bufferView": 1, "componentType": 5121},
			"values":  map[string]any{"bufferView": 2},
		},
	})
	return doc, bin, idxAbs
}

func TestSparseValid(t *testing.T) {
	doc, bin, _ := sparseFixture()
	expectOK(t, CheckBytes(makeGLB(t, doc, bin)))
}

// 重复索引必须拒绝。
func TestSparseDuplicateIndex(t *testing.T) {
	doc, bin, idxAbs := sparseFixture()
	bin[idxAbs+1] = 1 // [1,1,9]
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.indexNotIncreasing")
}

// 倒序索引必须拒绝。
func TestSparseDescendingIndex(t *testing.T) {
	doc, bin, idxAbs := sparseFixture()
	bin[idxAbs+0] = 9
	bin[idxAbs+1] = 5
	bin[idxAbs+2] = 1 // [9,5,1]
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.indexNotIncreasing")
}

// 索引越界（>= count）必须拒绝。
func TestSparseIndexOutOfRange(t *testing.T) {
	doc, bin, idxAbs := sparseFixture()
	bin[idxAbs+2] = 10 // count=10，合法索引最大为 9
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.indexOutOfRange")
}

// sparse count 超过访问器 count 必须拒绝。
func TestSparseCountOverflow(t *testing.T) {
	doc, bin, _ := sparseFixture()
	acc := doc["accessors"].([]any)[0].(map[string]any)
	acc["sparse"].(map[string]any)["count"] = 11
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.countOverflow")
}

// indices 仅支持 5121/5123/5125。
func TestSparseBadIndexComponentType(t *testing.T) {
	doc, bin, _ := sparseFixture()
	acc := doc["accessors"].([]any)[0].(map[string]any)
	acc["sparse"].(map[string]any)["indices"].(map[string]any)["componentType"] = 5120
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.badIndexType")
}

// uint16 与 uint32 索引类型应合法读取。
func TestSparseUint16Uint32Indices(t *testing.T) {
	// uint16 索引 [2,4,6]，6 字节。
	bin := make([]byte, 64)
	bin[40], bin[41] = 2, 0
	bin[42], bin[43] = 4, 0
	bin[44], bin[45] = 6, 0
	doc := baseDoc(64)
	withViews(doc, view(0, 40, nil), view(40, 8, nil), view(48, 12, nil))
	withAccessors(doc, map[string]any{
		"bufferView": 0, "componentType": 5126, "count": 10, "type": "SCALAR",
		"sparse": map[string]any{
			"count":   3,
			"indices": map[string]any{"bufferView": 1, "componentType": 5123},
			"values":  map[string]any{"bufferView": 2},
		},
	})
	expectOK(t, CheckBytes(makeGLB(t, doc, bin)))

	// uint32 索引 [3,6,9]，12 字节。
	bin = make([]byte, 64)
	for i, v := range []uint32{3, 6, 9} {
		off := 40 + i*4
		bin[off] = byte(v)
	}
	doc = baseDoc(64)
	withViews(doc, view(0, 40, nil), view(40, 12, nil), view(52, 12, nil))
	withAccessors(doc, map[string]any{
		"bufferView": 0, "componentType": 5126, "count": 10, "type": "SCALAR",
		"sparse": map[string]any{
			"count":   3,
			"indices": map[string]any{"bufferView": 1, "componentType": 5125},
			"values":  map[string]any{"bufferView": 2},
		},
	})
	expectOK(t, CheckBytes(makeGLB(t, doc, bin)))
}

// indices / values 视图声明 byteStride 必须拒绝。
func TestSparseViewStrideNotAllowed(t *testing.T) {
	doc, bin, _ := sparseFixture()
	views := doc["bufferViews"].([]any)
	views[1].(map[string]any)["byteStride"] = 4
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.strideNotAllowed")

	doc, bin, _ = sparseFixture()
	views = doc["bufferViews"].([]any)
	views[2].(map[string]any)["byteStride"] = 4
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.strideNotAllowed")
}

// indices 偏移未对齐必须拒绝（uint16 索引偏移 1）。
func TestSparseIndicesMisaligned(t *testing.T) {
	doc, bin, _ := sparseFixture()
	acc := doc["accessors"].([]any)[0].(map[string]any)
	indices := acc["sparse"].(map[string]any)["indices"].(map[string]any)
	indices["componentType"] = 5123
	indices["byteOffset"] = 1
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.offsetMisaligned")
}

// indices 数据越过视图边界必须拒绝。
func TestSparseIndicesOutOfView(t *testing.T) {
	doc, bin, _ := sparseFixture()
	acc := doc["accessors"].([]any)[0].(map[string]any)
	// 视图 1 长 4 字节，uint16 索引 3 个需 6 字节。
	indices := acc["sparse"].(map[string]any)["indices"].(map[string]any)
	indices["componentType"] = 5123
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.indicesOutOfView")
}

// values 数据越过视图边界必须拒绝（少 1 字节）。
func TestSparseValuesOutOfView(t *testing.T) {
	doc, bin, _ := sparseFixture()
	views := doc["bufferViews"].([]any)
	views[2].(map[string]any)["byteLength"] = 11 // 3 个 float32 需 12 字节
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.valuesOutOfView")
}

// values 遵循矩阵布局：MAT3 int8，sparse count=2，紧密占用 12+11=23 字节。
func TestSparseValuesMatrixLayout(t *testing.T) {
	bin := make([]byte, 32)
	bin[0] = 1
	bin[1] = 7 // uint8 索引 [1,7]
	doc := baseDoc(32)
	withViews(doc,
		view(0, 4, nil),  // 0: indices
		view(4, 23, nil), // 1: values，恰好 23 字节
	)
	withAccessors(doc, map[string]any{
		"componentType": 5120, "count": 8, "type": "MAT3", // 无 bufferView，全零底值
		"sparse": map[string]any{
			"count":   2,
			"indices": map[string]any{"bufferView": 0, "componentType": 5121},
			"values":  map[string]any{"bufferView": 1},
		},
	})
	expectOK(t, CheckBytes(makeGLB(t, doc, bin)))

	// values 视图少 1 字节必须拒绝。
	bin = make([]byte, 32)
	bin[0] = 1
	bin[1] = 7
	doc = baseDoc(32)
	withViews(doc, view(0, 4, nil), view(4, 22, nil))
	withAccessors(doc, map[string]any{
		"componentType": 5120, "count": 8, "type": "MAT3",
		"sparse": map[string]any{
			"count":   2,
			"indices": map[string]any{"bufferView": 0, "componentType": 5121},
			"values":  map[string]any{"bufferView": 1},
		},
	})
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.valuesOutOfView")
}

// values 矩阵绝对起点未 4 字节对齐必须拒绝。
func TestSparseValuesMatrixMisaligned(t *testing.T) {
	bin := make([]byte, 32)
	bin[0] = 1
	bin[1] = 7
	doc := baseDoc(32)
	withViews(doc, view(0, 4, nil), view(5, 23, nil)) // 起点 5，未 4 对齐
	withAccessors(doc, map[string]any{
		"componentType": 5120, "count": 8, "type": "MAT3",
		"sparse": map[string]any{
			"count":   2,
			"indices": map[string]any{"bufferView": 0, "componentType": 5121},
			"values":  map[string]any{"bufferView": 1},
		},
	})
	expectCode(t, CheckBytes(makeGLB(t, doc, bin)), "sparse.columnMisaligned")
}
