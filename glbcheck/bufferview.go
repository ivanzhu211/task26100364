package glbcheck

// checkBuffers 校验 buffer 声明与 BIN 块的关系：
// 最多一个内嵌 buffer，不允许外部 URI；BIN 只允许比 byteLength 多 0~3 字节零填充。
// 返回声明的 buffer 长度；不可继续时返回 ok=false。
func checkBuffers(g *gltf, bin []byte, rep *Report) (declaredLen int64, ok bool) {
	if len(g.Buffers) > 1 {
		rep.errorf("buffer.tooMany", "buffers", "声明了 %d 个 buffer，最多允许 1 个内嵌 buffer", len(g.Buffers))
		return 0, false
	}
	if len(g.Buffers) == 0 {
		if bin != nil {
			rep.errorf("buffer.binWithoutBuffer", "buffers", "存在 BIN 块但未声明任何 buffer")
			return 0, false
		}
		return 0, true
	}
	buf := g.Buffers[0]
	if buf.URI != "" {
		rep.errorf("buffer.externalURI", "buffers[0].uri", "不支持外部 URI buffer: %q", buf.URI)
		return 0, false
	}
	if buf.ByteLength < 1 {
		rep.errorf("buffer.badLength", "buffers[0].byteLength", "byteLength %d 必须为正整数", buf.ByteLength)
		return 0, false
	}
	if bin == nil {
		rep.errorf("buffer.missingBIN", "buffers[0]", "声明了 byteLength=%d 的内嵌 buffer 但缺少 BIN 块", buf.ByteLength)
		return 0, false
	}
	binLen := int64(len(bin))
	if binLen < buf.ByteLength {
		rep.errorf("buffer.binTooShort", "buffers[0].byteLength", "BIN 块长度 %d 小于声明的 byteLength %d", binLen, buf.ByteLength)
		return 0, false
	}
	pad := binLen - buf.ByteLength
	if pad > 3 {
		rep.errorf("buffer.excessPadding", "buffers[0].byteLength", "BIN 块比 byteLength 多 %d 字节，超过允许的 3 字节填充", pad)
		return 0, false
	}
	for i := buf.ByteLength; i < binLen; i++ {
		if bin[i] != 0 {
			rep.errorf("buffer.nonZeroPadding", "buffers[0]", "BIN 块填充区偏移 %d 处字节 0x%02X 非零", i, bin[i])
			return 0, false
		}
	}
	return buf.ByteLength, true
}

// checkBufferViews 校验全部 bufferView 的字节范围与步长基本约束。
// 所有 bufferView 均以声明的 buffer 长度为边界。
// 返回每个视图的合法性标记，供 accessor 检查避免级联报错。
func checkBufferViews(g *gltf, declaredLen int64, rep *Report) []bool {
	valid := make([]bool, len(g.BufferViews))
	for i, bv := range g.BufferViews {
		path := sprintf("bufferViews[%d]", i)
		if bv.Buffer != 0 {
			rep.errorf("bufferView.badBuffer", path+".buffer", "引用 buffer %d，但仅允许内嵌 buffer 0", bv.Buffer)
			continue
		}
		if bv.ByteOffset < 0 {
			rep.errorf("bufferView.badOffset", path+".byteOffset", "byteOffset %d 不能为负", bv.ByteOffset)
			continue
		}
		if bv.ByteLength < 1 {
			rep.errorf("bufferView.badLength", path+".byteLength", "byteLength %d 必须为正整数", bv.ByteLength)
			continue
		}
		if bv.ByteOffset+bv.ByteLength > declaredLen {
			rep.errorf("bufferView.outOfRange", path, "范围 [%d, %d) 超出声明的 buffer 长度 %d",
				bv.ByteOffset, bv.ByteOffset+bv.ByteLength, declaredLen)
			continue
		}
		if bv.ByteStride != nil {
			stride := *bv.ByteStride
			if stride < 4 || stride > 252 || stride%4 != 0 {
				rep.errorf("bufferView.badStride", path+".byteStride", "byteStride %d 必须是 4 至 252 之间 4 的倍数", stride)
				continue
			}
		}
		valid[i] = true
	}
	return valid
}
