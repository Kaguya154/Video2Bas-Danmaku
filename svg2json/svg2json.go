package svg2json

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/color"
	v2btypes "src/type"
	"strings"
	"sync"
)

// ParseAllFrame 并发解析所有 FrameSVG
func ParseAllFrame(frames []v2btypes.FrameSVG) []v2btypes.FrameData {
	return ParseAllFrameWithParallel(frames, 4)
}

// ParseAllFrameWithParallel 并发解析所有 FrameSVG，带并发上限
func ParseAllFrameWithParallel(frames []v2btypes.FrameSVG, parallel int) []v2btypes.FrameData {
	results := make([]v2btypes.FrameData, len(frames))
	var wg sync.WaitGroup
	if parallel <= 0 {
		parallel = 1
	}
	sem := make(chan struct{}, parallel)

	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.FrameSVG) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			layers := ParseFrame(frame)
			results[idx] = layers
		}(i, f)
	}

	wg.Wait()
	return results
}

// ParseFrame 解析单帧 SVG
func ParseFrame(frame v2btypes.FrameSVG) v2btypes.FrameData {
	result := make([]map[string]string, 0, len(frame.Layers))

	for _, layer := range frame.Layers {
		paths := extractPaths(layer.SVGData)
		data := map[string]string{
			"color":    fmt.Sprintf("%s", toHex(layer.Color)),
			"pathdata": strings.Join(paths, " "),
		}
		result = append(result, data)
	}

	return v2btypes.FrameData{
		FrameIndex: frame.FrameIndex,
		Data:       result,
	}
}

func toHex(c color.RGBA) string {
	return fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B)
}

// ParseFrameJSON 返回 JSON 字符串
func ParseFrameJSON(frame v2btypes.FrameSVG) ([]byte, error) {
	fd := ParseFrame(frame)
	return json.MarshalIndent([]v2btypes.FrameData{fd}, "", "  ")
}

// extractPaths 从 SVG 字符串中递归提取所有 <path> 的 d 属性
func extractPaths(svgStr string) []string {
	decoder := xml.NewDecoder(strings.NewReader(svgStr))
	paths := []string{}

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "path" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "d" {
						paths = append(paths, attr.Value)
					}
				}
			}
		}
	}

	return paths
}
