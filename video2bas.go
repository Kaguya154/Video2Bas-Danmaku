package main

import (
	"context"
	"log"
	"os"
	"src/color2svg"
	"src/json2bas"
	"src/svg2json"
	video2color "src/video2color"
	"strconv"
	"strings"

	"github.com/rustyoz/svg"
)

func generateBasToFile(ctx context.Context, videoPath string, fps, maxWidth, colorCount, maxFileSize int, outputPath string) {
	basLines := generateBas(ctx, videoPath, fps, maxWidth, colorCount)
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
			currentFile, err = os.Create(outputPath + "_" + strconv.Itoa(fileId) + ".bas")
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
}

func generateBas(ctx context.Context, videoPath string, fps, maxWidth, colorCount int) []string {
	log.Println("Extracting frames from video...")
	frames, err := video2color.ExtractFrames(ctx, videoPath, fps, maxWidth)
	if err != nil {
		log.Println("Error extracting frames:")
		log.Fatal(err)
	}
	log.Printf("Extracted %d frames\n", len(frames))
	frameLayers, err := video2color.SplitAllFramesAuto(frames, colorCount)
	if err != nil {
		log.Println("Error extracting frames:")
		log.Fatal(err)
	}
	log.Println("Converting frames to SVG...")
	svgLayers, err := color2svg.ConvertToSVG(frameLayers)
	if err != nil {
		log.Println("Error converting frames:")
		log.Fatal(err)
	}
	data := svg2json.ParseAllFrame(svgLayers)
	svgData := svgLayers[0].Layers[0].SVGData
	parsed, err := svg.ParseSvg(svgData, "example", 1.0)
	if err != nil {
		log.Fatal(err)
	}
	box := parsed.ViewBox
	//从box读取4个float64
	split := strings.Split(box, " ")
	ints := make([]int, 4)
	for idx, s := range split {
		ints[idx], _ = strconv.Atoi(s)
	}
	log.Println("Generating BAS code...")
	return json2bas.GenerateAllBasText(data, ints[2]*10, ints[3]*10, float64(fps), 0)
}
