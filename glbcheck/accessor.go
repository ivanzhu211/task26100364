package glbcheck

import "encoding/binary"

// componentSize 返回分量类型的字节数。
func componentSize(componentType int64) (int64, bool) {
	switch componentType {
	case 5120, 5121: // int8, uint8
		return 1, true
	case 5122, 5123: // int16, uint16
		return 2, true
	case 5125, 5126: // uint32, float32
		return 4, true
	}
	return 0, false
}

// shapeInfo 返回元素形状的列数与行数（每列分量数）。
// 标量与向量视为单列。
func shapeInfo(typ string) (cols, rows int64, ok bool) {
	switch typ {
	case "SCALAR":
		return 1, 1, true
	case "VEC2":
		return 1, 2, true
	case "VEC3":
		return 1, 3, true
	case "VEC4":
		return 1, 4, true
	case "MAT2":
		return 2, 2, true
	case "MAT3":
		return 3, 3, true
	case "MAT4":
		return 4, 4, true
	}
	return 0, 0, false
}

func align4(n int64) int64 { return (n + 3) &^ 3 }

// layout 描述一种 componentType + type 组合的字节布局。
type layout struct {
	compSize    int64 // 分量字节数
	matrix      bool
	colStride   int64 // 矩阵列步长（列起点按 4 字节对齐）
	tightStride int64 // 紧密布局下完整元素步长（含列填充）
	lastSize    int64 // 最后一个元素实际占用的字节数（末列填充可省略）
}

func makeLayout(compSize, cols, rows int64) layout {
	l := layout{compSize: compSize}
	if cols == 1 {
		l.tightStride = rows * compSize
		l.lastSize = rows * compSize
		return l
	}
	l.matrix = true
	l.colStride = align4(rows * compSize)
	l.tightStride = cols * l.colStride
	l.lastSize = (cols-1)*l.colStride + rows*compSize
	return l
}

// checkAccessors 校验全部 accessor 及其 sparse 引用的布局与范围。
// viewsValid 标记每个 bufferView 是否通过校验，无效视图引发的级联错误不再重复报告。
func checkAccessors(g *gltf, bin []byte, viewsValid []bool, rep *Report) {
	for i, acc := range g.Accessors {
		path := sprintf("accessors[%d]", i)
		compSize, ok := componentSize(acc.ComponentType)
		if !ok {
			rep.errorf("accessor.badComponentType", path+".componentType", "componentType %d 不受支持", acc.ComponentType)
			continue
		}
		cols, rows, ok := shapeInfo(acc.Type)
		if !ok {
			rep.errorf("accessor.badType", path+".type", "type %q 不受支持", acc.Type)
			continue
		}
		if acc.Count < 1 {
			rep.errorf("accessor.badCount", path+".count", "count %d 必须为正整数", acc.Count)
			continue
		}
		if acc.ByteOffset < 0 {
			rep.errorf("accessor.badOffset", path+".byteOffset", "byteOffset %d 不能为负", acc.ByteOffset)
			continue
		}
		lay := makeLayout(compSize, cols, rows)

		if acc.BufferView == nil {
			// 无 bufferView：以全零为底值，byteOffset 只能缺省或为 0。
			if acc.ByteOffset != 0 {
				rep.errorf("accessor.zeroBaseOffset", path+".byteOffset", "无 bufferView 的访问器 byteOffset 只能缺省或为 0，得到 %d", acc.ByteOffset)
			}
		} else {
			viewIdx := *acc.BufferView
			if viewIdx < 0 || viewIdx >= len(g.BufferViews) {
				rep.errorf("accessor.badBufferView", path+".bufferView", "引用不存在的 bufferView %d", viewIdx)
				continue
			}
			if !viewsValid[viewIdx] {
				continue // 视图本身已报错，跳过级联检查
			}
			view := g.BufferViews[viewIdx]
			checkAccessorLayout(&acc, &view, lay, path, rep)
		}

		if acc.Sparse != nil {
			checkSparse(&acc, acc.Sparse, g, bin, viewsValid, lay, path+".sparse", rep)
		}
	}
}

// checkAccessorLayout 校验访问器在视图内的偏移、对齐与占用范围。
func checkAccessorLayout(acc *accessor, view *bufferView, lay layout, path string, rep *Report) {
	if acc.ByteOffset%lay.compSize != 0 {
		rep.errorf("accessor.offsetMisaligned", path+".byteOffset",
			"相对视图偏移 %d 未按分量大小 %d 对齐", acc.ByteOffset, lay.compSize)
	}
	absOffset := view.ByteOffset + acc.ByteOffset
	if absOffset%lay.compSize != 0 {
		rep.errorf("accessor.absOffsetMisaligned", path,
			"绝对偏移 %d 未按分量大小 %d 对齐", absOffset, lay.compSize)
	}
	if lay.matrix && absOffset%4 != 0 {
		rep.errorf("accessor.columnMisaligned", path,
			"矩阵元素绝对起点 %d 未按 4 字节对齐，列起点无法对齐", absOffset)
	}
	stride := lay.tightStride
	if view.ByteStride != nil {
		stride = *view.ByteStride
		if stride < lay.tightStride {
			rep.errorf("accessor.strideTooSmall", path,
				"byteStride %d 小于完整布局步长 %d", stride, lay.tightStride)
			return
		}
	}
	occupied := (acc.Count-1)*stride + lay.lastSize
	if acc.ByteOffset+occupied > view.ByteLength {
		rep.errorf("accessor.outOfView", path,
			"末尾元素实际占用结束于视图内偏移 %d，超出视图长度 %d",
			acc.ByteOffset+occupied, view.ByteLength)
	}
}

// checkSparse 校验 sparse 覆盖的索引与值。
func checkSparse(acc *accessor, sp *sparse, g *gltf, bin []byte, viewsValid []bool, lay layout, path string, rep *Report) {
	if sp.Count < 1 {
		rep.errorf("sparse.badCount", path+".count", "sparse count %d 必须为正整数", sp.Count)
		return
	}
	if sp.Count > acc.Count {
		rep.errorf("sparse.countOverflow", path+".count", "sparse count %d 超过访问器 count %d", sp.Count, acc.Count)
		return
	}
	checkSparseIndices(acc, sp, g, bin, viewsValid, path+".indices", rep)
	checkSparseValues(sp, g, viewsValid, lay, path+".values", rep)
}

// sparseView 取出 sparse 子对象引用的合法视图；view 无 byteStride 是强制要求。
func sparseView(viewIdx int, g *gltf, viewsValid []bool, path string, rep *Report) (*bufferView, bool) {
	if viewIdx < 0 || viewIdx >= len(g.BufferViews) {
		rep.errorf("sparse.badBufferView", path+".bufferView", "引用不存在的 bufferView %d", viewIdx)
		return nil, false
	}
	if !viewsValid[viewIdx] {
		return nil, false
	}
	view := g.BufferViews[viewIdx]
	if view.ByteStride != nil {
		rep.errorf("sparse.strideNotAllowed", path+".bufferView", "sparse 视图不得声明 byteStride")
		return nil, false
	}
	return &view, true
}

func checkSparseIndices(acc *accessor, sp *sparse, g *gltf, bin []byte, viewsValid []bool, path string, rep *Report) {
	idxCompSize, ok := componentSize(sp.Indices.ComponentType)
	if !ok || sp.Indices.ComponentType == 5120 || sp.Indices.ComponentType == 5122 || sp.Indices.ComponentType == 5126 {
		rep.errorf("sparse.badIndexType", path+".componentType",
			"sparse indices componentType %d 不受支持，仅允许 5121/5123/5125", sp.Indices.ComponentType)
		return
	}
	if sp.Indices.ByteOffset < 0 {
		rep.errorf("sparse.badOffset", path+".byteOffset", "byteOffset %d 不能为负", sp.Indices.ByteOffset)
		return
	}
	view, ok := sparseView(sp.Indices.BufferView, g, viewsValid, path, rep)
	if !ok {
		return
	}
	if sp.Indices.ByteOffset%idxCompSize != 0 {
		rep.errorf("sparse.offsetMisaligned", path+".byteOffset",
			"相对视图偏移 %d 未按分量大小 %d 对齐", sp.Indices.ByteOffset, idxCompSize)
	}
	absOffset := view.ByteOffset + sp.Indices.ByteOffset
	if absOffset%idxCompSize != 0 {
		rep.errorf("sparse.absOffsetMisaligned", path,
			"绝对偏移 %d 未按分量大小 %d 对齐", absOffset, idxCompSize)
	}
	need := sp.Indices.ByteOffset + sp.Count*idxCompSize
	if need > view.ByteLength {
		rep.errorf("sparse.indicesOutOfView", path,
			"索引数据结束于视图内偏移 %d，超出视图长度 %d", need, view.ByteLength)
		return
	}
	// 读取索引值：必须严格递增且小于访问器 count。
	if bin == nil {
		return
	}
	base := absOffset
	prev := int64(-1)
	for i := int64(0); i < sp.Count; i++ {
		off := base + i*idxCompSize
		var v int64
		switch idxCompSize {
		case 1:
			v = int64(bin[off])
		case 2:
			v = int64(binary.LittleEndian.Uint16(bin[off:]))
		default:
			v = int64(binary.LittleEndian.Uint32(bin[off:]))
		}
		if v >= acc.Count {
			rep.errorf("sparse.indexOutOfRange", path, "第 %d 个稀疏索引 %d 不小于访问器 count %d", i, v, acc.Count)
			return
		}
		if v <= prev {
			rep.errorf("sparse.indexNotIncreasing", path, "第 %d 个稀疏索引 %d 未严格大于前一个 %d", i, v, prev)
			return
		}
		prev = v
	}
}

func checkSparseValues(sp *sparse, g *gltf, viewsValid []bool, lay layout, path string, rep *Report) {
	if sp.Values.ByteOffset < 0 {
		rep.errorf("sparse.badOffset", path+".byteOffset", "byteOffset %d 不能为负", sp.Values.ByteOffset)
		return
	}
	view, ok := sparseView(sp.Values.BufferView, g, viewsValid, path, rep)
	if !ok {
		return
	}
	if sp.Values.ByteOffset%lay.compSize != 0 {
		rep.errorf("sparse.offsetMisaligned", path+".byteOffset",
			"相对视图偏移 %d 未按分量大小 %d 对齐", sp.Values.ByteOffset, lay.compSize)
	}
	absOffset := view.ByteOffset + sp.Values.ByteOffset
	if absOffset%lay.compSize != 0 {
		rep.errorf("sparse.absOffsetMisaligned", path,
			"绝对偏移 %d 未按分量大小 %d 对齐", absOffset, lay.compSize)
	}
	if lay.matrix && absOffset%4 != 0 {
		rep.errorf("sparse.columnMisaligned", path,
			"矩阵值绝对起点 %d 未按 4 字节对齐", absOffset)
	}
	// values 视图不得有 byteStride，元素按紧密布局排列。
	occupied := (sp.Count-1)*lay.tightStride + lay.lastSize
	if sp.Values.ByteOffset+occupied > view.ByteLength {
		rep.errorf("sparse.valuesOutOfView", path,
			"值数据结束于视图内偏移 %d，超出视图长度 %d", sp.Values.ByteOffset+occupied, view.ByteLength)
	}
}
