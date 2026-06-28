// Package main 嵌入 Vue 编译后的静态资源
package main

import (
	"embed"
)

// EmbeddedAssets 嵌入 ./assets 目录下的所有文件
//
//go:embed assets
var EmbeddedAssets embed.FS
