package glbcheck

import "encoding/binary"

// 本文件负责 GLB 容器层解析：12 字节头、JSON 块、可选 BIN 块。

const (
	glbMagic   = 0x46546C67 // "glTF"
	glbVersion = 2

	chunkTypeJSON = 0x4E4F534A // "JSON"
	chunkTypeBIN  = 0x004E4942 // "BIN\0"

	headerSize = 12
	chunkHead  = 8
)

// container 是解析后的 GLB 容器。
type container struct {
	json []byte // JSON 块内容
	bin  []byte // BIN 块内容，可为 nil
}

// parseContainer 解析并校验 GLB 容器结构，错误追加到 res。
// 结构非法时返回 nil。
func parseContainer(data []byte, res *Result) *container {
	if len(data) < headerSize {
		res.errorf("header", "CONTAINER_TRUNCATED", "文件长度 %d 小于 GLB 头部长度 %d", len(data), headerSize)
		return nil
	}
	if magic := binary.LittleEndian.Uint32(data[0:4]); magic != glbMagic {
		res.errorf("header.magic", "CONTAINER_MAGIC", "魔数错误: 0x%08X，期望 0x%08X (glTF)", magic, glbMagic)
		return nil
	}
	if version := binary.LittleEndian.Uint32(data[4:8]); version != glbVersion {
		res.errorf("header.version", "CONTAINER_VERSION", "不支持的 GLB 版本: %d，仅支持 2", version)
		return nil
	}
	if total := binary.LittleEndian.Uint32(data[8:12]); int(total) != len(data) {
		res.errorf("header.length", "CONTAINER_LENGTH", "头部声明总长度 %d 与实际长度 %d 不一致", total, len(data))
		return nil
	}

	c := &container{}
	offset := headerSize
	chunkIndex := 0
	for offset < len(data) {
		if len(data)-offset < chunkHead {
			res.errorf("chunks", "CHUNK_TRUNCATED", "偏移 %d 处剩余 %d 字节，不足块头长度 %d", offset, len(data)-offset, chunkHead)
			return nil
		}
		chunkLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		if chunkLen%4 != 0 {
			res.errorf("chunks", "CHUNK_ALIGNMENT", "偏移 %d 处块长度 %d 未按 4 字节对齐", offset, chunkLen)
			return nil
		}
		bodyStart := offset + chunkHead
		bodyEnd := bodyStart + int(chunkLen)
		if bodyEnd > len(data) {
			res.errorf("chunks", "CHUNK_LENGTH", "偏移 %d 处块声明长度 %d 超出文件末尾", offset, chunkLen)
			return nil
		}
		body := data[bodyStart:bodyEnd]
		switch chunkType {
		case chunkTypeJSON:
			if chunkIndex != 0 {
				res.errorf("chunks", "CHUNK_ORDER", "JSON 块必须是第一个块")
				return nil
			}
			if c.json != nil {
				res.errorf("chunks", "CHUNK_ORDER", "存在多个 JSON 块")
				return nil
			}
			c.json = body
		case chunkTypeBIN:
			if chunkIndex == 0 {
				res.errorf("chunks", "CHUNK_ORDER", "首个块必须是 JSON 块")
				return nil
			}
			if c.bin != nil {
				res.errorf("chunks", "CHUNK_ORDER", "存在多个 BIN 块")
				return nil
			}
			c.bin = body
		default:
			res.errorf("chunks", "CHUNK_TYPE", "不支持的块类型: 0x%08X", chunkType)
			return nil
		}
		offset = bodyEnd
		chunkIndex++
	}
	if c.json == nil {
		res.errorf("chunks", "CHUNK_ORDER", "缺少 JSON 块")
		return nil
	}
	return c
}
