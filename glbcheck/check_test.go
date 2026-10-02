package glbcheck

import (
	"os"
	"path/filepath"
	"testing"
)

// CheckFile 从本地目录读取源文件，结果仅保存在内存。
func TestCheckFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.glb")
	data := makeGLB(t, baseDoc(4), make([]byte, 4))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	rep, err := CheckFile(path)
	if err != nil {
		t.Fatalf("CheckFile 返回错误: %v", err)
	}
	expectOK(t, rep)
}

func TestCheckFileNotExist(t *testing.T) {
	if _, err := CheckFile(filepath.Join(t.TempDir(), "missing.glb")); err == nil {
		t.Fatal("期望对不存在的文件返回错误")
	}
}
