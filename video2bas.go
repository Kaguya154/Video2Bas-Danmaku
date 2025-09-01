package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"video2bas/color2svg"
	"video2bas/json2bas"
	"video2bas/svg2json"
	v2btypes "video2bas/type"
	"video2bas/video2color"

	"github.com/rustyoz/svg"
)

func generateBasToFile(ctx context.Context, videoPath string, fps, maxWidth, colorCount, maxFileSize int, outputPath string, parallel int) {
	basLines := generateBas(ctx, videoPath, fps, maxWidth, colorCount, parallel)

	//检查outputPath的目录是否存在，不存在则创建
	if strings.Contains(outputPath, "/") {
		dir := strings.TrimRight(outputPath, "/")
		if dir != "" {
			dir = dir[:strings.LastIndex(dir, "/")]
			if dir != "" {
				err := os.MkdirAll(dir, os.ModePerm)
				log.Println("Output directory:", dir)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	fileId := 0
	currentFileSize := 0
	var currentFile *os.File
	var err error
	for _, line := range basLines {
		lineSize := len(line) + 1 // +1 for newline
		if currentFile == nil || currentFileSize+lineSize > maxFileSize {
			if currentFile != nil {
				currentFile.Close()
			}
			currentFile, err = os.Create(outputPath + "_" + strconv.Itoa(fileId) + ".bas.txt")
			if err != nil {
				log.Fatal(err)
			}
			fileId++
			currentFileSize = 0
		}
		_, err := currentFile.WriteString(line + "\n")
		if err != nil {
			log.Fatal(err)
		}
		currentFileSize += lineSize
	}
	if currentFile != nil {
		currentFile.Close()
	}
	log.Println("Output Bas files count:", fileId)
}

func generateBas(ctx context.Context, videoPath string, fps, maxWidth, colorCount int, parallel int) []string {
	log.Println("Extracting frames from video...")
	frames, err := video2color.ExtractFrames(ctx, videoPath, fps, maxWidth)
	if err != nil {
		log.Println("Error extracting frames:")
		log.Fatal(err)
	}
	log.Printf("Extracted %d frames\n", len(frames))

	log.Println("Splitting frames into color layers...")
	// 进度监控：分层
	frameLayers := make([]v2btypes.FrameLayers, len(frames))
	var splitDoneCount int
	splitDoneCh := make(chan int, len(frames))
	stopSplitProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("[Progress] Splitting: %d/%d", splitDoneCount, len(frames))
			case <-stopSplitProgress:
				return
			}
		}
	}()
	frameLayers, err = video2color.SplitAllFramesAutoWithProgress(frames, colorCount, parallel, func() {
		splitDoneCount++
		splitDoneCh <- 1
	})
	close(stopSplitProgress)
	if err != nil {
		log.Println("Error extracting frames:")
		log.Fatal(err)
	}
	log.Println("Converting frames to SVG...")

	// 进度监控：SVG
	svgLayers := make([]v2btypes.FrameSVG, len(frameLayers))
	var svgDoneCount int
	stopSvgProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("[Progress] Converting SVG: %d/%d", svgDoneCount, len(frameLayers))
			case <-stopSvgProgress:
				return
			}
		}
	}()
	svgLayers, err = color2svg.ConvertToSVGWithProgress(frameLayers, func() {
		svgDoneCount++
	})
	close(stopSvgProgress)
	if err != nil {
		log.Println("Error converting frames:")
		log.Fatal(err)
	}

	// 进度监控：SVG2JSON
	data := make([]v2btypes.FrameData, len(svgLayers))
	var jsonDoneCount int
	stopJsonProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("[Progress] Transform svg data: %d/%d", jsonDoneCount, len(svgLayers))
			case <-stopJsonProgress:
				return
			}
		}
	}()
	data = svg2json.ParseAllFrameWithParallelProgress(svgLayers, parallel, func() {
		jsonDoneCount++
	})
	close(stopJsonProgress)

	svgData := svgLayers[0].Layers[0].SVGData
	parsed, err := svg.ParseSvg(svgData, "example", 1.0)
	if err != nil {
		log.Fatal(err)
	}
	box := parsed.ViewBox
	//从box读取4个float64
	split := strings.Split(box, " ")
	floats := make([]float64, 4)
	for idx, s := range split {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			log.Fatal(err)
		}
		floats[idx] = f
	}
	width := int(floats[2] * 10)
	height := int(floats[3] * 10)

	log.Println("Generating BAS code...")
	return json2bas.GenerateAllBasTextWithParallel(data, width, height, float64(fps), 0, parallel)
}
