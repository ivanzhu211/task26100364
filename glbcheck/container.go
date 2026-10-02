package glbcheck

import "encoding/binary"

const (
	glbMagic       = 0x46546C67 // "glTF"
	glbVersion2    = 2
	chunkTypeJSON  = 0x4E4F534A // "JSON"
	chunkTypeBIN   = 0x004E4942 // "BIN\x00"
	glbHeaderLen   = 12
	chunkHeaderLen = 8
)

// container 是解析后的 GLB 容器内容。
type container struct {
	jsonChunk []byte
	binChunk  []byte // 可为 nil，表示没有 BIN 块
}

// parseContainer 校验 GLB 头与块布局，返回 JSON 与 BIN 块内容。
// 校验失败时返回 nil（问题已记入 rep）。
func parseContainer(data []byte, rep *Report) *container {
	if len(data) < glbHeaderLen {
		rep.errorf("container.tooShort", "", "文件长度 %d 小于 GLB 头长度 %d", len(data), glbHeaderLen)
		return nil
	}
	if magic := binary.LittleEndian.Uint32(data[0:4]); magic != glbMagic {
		rep.errorf("container.badMagic", "", "魔数 0x%08X 不是 glTF", magic)
		return nil
	}
	if version := binary.LittleEndian.Uint32(data[4:8]); version != glbVersion2 {
		rep.errorf("container.unsupportedVersion", "", "GLB 版本 %d 不受支持，仅支持 2", version)
		return nil
	}
	declaredLen := binary.LittleEndian.Uint32(data[8:12])
	if uint64(declaredLen) != uint64(len(data)) {
		rep.errorf("container.lengthMismatch", "", "GLB 声明总长度 %d 与实际长度 %d 不一致", declaredLen, len(data))
		return nil
	}

	c := &container{}
	offset := glbHeaderLen
	chunkIndex := 0
	for offset < len(data) {
		if len(data)-offset < chunkHeaderLen {
			rep.errorf("container.truncatedChunkHeader", "", "偏移 %d 处剩余 %d 字节，不足块头长度 %d", offset, len(data)-offset, chunkHeaderLen)
			return nil
		}
		chunkLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		if chunkLen%4 != 0 {
			rep.errorf("container.chunkAlignment", "", "第 %d 块长度 %d 不是 4 的倍数", chunkIndex, chunkLen)
			return nil
		}
		if uint64(chunkLen) > uint64(len(data)-offset-chunkHeaderLen) {
			rep.errorf("container.chunkOverflow", "", "第 %d 块长度 %d 超出剩余数据 %d 字节", chunkIndex, chunkLen, len(data)-offset-chunkHeaderLen)
			return nil
		}
		body := data[offset+chunkHeaderLen : offset+chunkHeaderLen+int(chunkLen)]
		switch chunkIndex {
		case 0:
			if chunkType != chunkTypeJSON {
				rep.errorf("container.firstChunkNotJSON", "", "首块类型 0x%08X 不是 JSON", chunkType)
				return nil
			}
			c.jsonChunk = body
		case 1:
			if chunkType != chunkTypeBIN {
				rep.errorf("container.unexpectedChunk", "", "第二块类型 0x%08X 不是 BIN（不支持扩展块）", chunkType)
				return nil
			}
			c.binChunk = body
		default:
			rep.errorf("container.tooManyChunks", "", "存在第 %d 块，GLB 最多允许 JSON 与 BIN 两块", chunkIndex+1)
			return nil
		}
		offset += chunkHeaderLen + int(chunkLen)
		chunkIndex++
	}
	if c.jsonChunk == nil {
		rep.errorf("container.missingJSON", "", "缺少 JSON 块")
		return nil
	}
	return c
}
