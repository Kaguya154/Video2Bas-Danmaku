package svg2json

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	v2btypes "src/type"
	"strings"
	"sync"
)

func ParseAllFrame(frames []v2btypes.FrameSVG) []v2btypes.FrameData {

	results := make([]v2btypes.FrameData, len(frames))

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.FrameSVG) {
			defer wg.Done()
			layers := ParseFrame(frame)
			results[idx] = layers
		}(i, f)
	}

	wg.Wait()

	return results
}

// ParseFrame 接收 FrameSVG，返回封装好的 FrameData
func ParseFrame(frame v2btypes.FrameSVG) v2btypes.FrameData {
	result := make([]map[string]string, 0, len(frame.Layers))

	for _, layer := range frame.Layers {
		paths := extractPaths(layer.SVGData)
		data := map[string]string{
			"color":    fmt.Sprintf("%d", layer.ColorIndex),
			"pathdata": strings.Join(paths, " "),
		}
		result = append(result, data)
	}

	return v2btypes.FrameData{
		FrameIndex: frame.FrameIndex,
		Data:       result,
	}
}

// ParseFrameJSON 返回 JSON 字符串
func ParseFrameJSON(frame v2btypes.FrameSVG) ([]byte, error) {
	fd := ParseFrame(frame)
	return json.MarshalIndent([]v2btypes.FrameData{fd}, "", "  ")
}

// extractPaths 从 SVG 字符串中提取所有 <path> 的 d 属性
func extractPaths(svg string) []string {
	type Path struct {
		D string `xml:"d,attr"`
	}

	type SVG struct {
		Paths []Path `xml:"path"`
	}

	var s SVG
	if err := xml.Unmarshal([]byte(svg), &s); err != nil {
		return nil
	}

	paths := make([]string, len(s.Paths))
	for i, p := range s.Paths {
		paths[i] = p.D
	}
	return paths
}
