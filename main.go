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
	savePath := flag.String("output", "output/video", "输出文件路径")
	maxFileSize := flag.Int("maxsize", 2*1024*1024, "单个输出文件最大尺寸，单位字节")
	parallel := flag.Int("parallel", 4, "并行处理的最大协程数")
	serial := flag.Bool("serial", false, "是否串行处理以最大程度减少内存使用")

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

	if *serial {
		generateBasToFileSerial(ctx, *videoPath, *fps, *maxWidth, *colorCount, *maxFileSize, *savePath)
	} else {
		generateBasToFile(ctx, *videoPath, *fps, *maxWidth, *colorCount, *maxFileSize, *savePath, *parallel)
	}
}
