package main

import (
	"context"
	"flag"
)

func main() {

	videoPath := flag.String("viedo", "", "视频文件路径")
	fps := flag.Int("fps", 10, "每秒帧数")
	maxWidth := flag.Int("width", 96, "最大宽度")
	colorCount := flag.Int("colors", 4, "颜色数量")
	savePath := flag.String("output", "output.bas", "输出文件路径")
	maxFileSize := flag.Int("maxsize", 100*1024, "单个输出文件最大尺寸，单位字节")

	help := flag.Bool("help", false, "显示帮助信息")
	flag.Parse()
	if *help {
		flag.Usage()
		return
	}
	if *videoPath == "" {
		flag.Usage()
		return
	}

	ctx := context.Background()

	generateBasToFile(ctx, *videoPath, *fps, *maxWidth, *colorCount, *maxFileSize, *savePath)
}
