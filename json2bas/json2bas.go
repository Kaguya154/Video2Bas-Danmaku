package json2bas

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	v2btypes "video2bas/type"
)

const (
	DefaultViewBoxW = 4000
	DefaultViewBoxH = 3620
)

// FlipSvgPath 类似 JS flipSvgPath
func FlipSvgPath(d string, viewBoxH int) string {
	if viewBoxH == 0 {
		viewBoxH = DefaultViewBoxH
	}

	re := regexp.MustCompile(`-?[0-9]*\.?[0-9]+(?:e[-+]?\d+)?|[MLHVCSQTAZmlhvcsqtaz]|[\s,]+`)
	tokens := re.FindAllString(d, -1)

	var output []string
	var command string
	var params []float64

	processGroup := func(group []float64, isAbs bool) []float64 {
		if len(group) == 0 {
			return group
		}
		switch command {
		case "V":
			return []float64{float64(viewBoxH) - group[0]}
		case "v":
			return []float64{-group[0]}
		case "A":
			return []float64{group[0], group[1], group[2], group[3], group[4], group[5], float64(viewBoxH) - group[6]}
		case "a":
			return []float64{group[0], group[1], group[2], group[3], group[4], group[5], -group[6]}
		default:
			res := make([]float64, len(group))
			for i, val := range group {
				if i%2 == 1 {
					if isAbs {
						res[i] = float64(viewBoxH) - val
					} else {
						res[i] = -val
					}
				} else {
					res[i] = val
				}
			}
			return res
		}
	}

	getGroupSize := func(cmd string) int {
		switch strings.ToUpper(cmd) {
		case "H", "h", "V", "v":
			return 1
		case "M", "m", "L", "l", "T", "t":
			return 2
		case "S", "s", "Q", "q":
			return 4
		case "C", "c":
			return 6
		case "A", "a":
			return 7
		case "Z", "z":
			return 0
		default:
			return 0
		}
	}

	for _, token := range tokens {
		t := strings.TrimSpace(token)
		if t == "" || t == "," {
			continue
		}
		if match, _ := regexp.MatchString(`^[MLHVCSQTAZmlhvcsqtaz]$`, t); match {
			if len(params) > 0 {
				groupSize := getGroupSize(command)
				for i := 0; i < len(params); i += groupSize {
					group := params[i:min(i+groupSize, len(params))]
					processed := processGroup(group, command == strings.ToUpper(command))
					strs := make([]string, len(processed))
					for j, v := range processed {
						strs[j] = strconv.FormatFloat(v, 'f', -1, 64)
					}
					output = append(output, strings.Join(strs, " "))
				}
				params = nil
			}
			command = t
			output = append(output, t)
		} else {
			num, _ := strconv.ParseFloat(t, 64)
			params = append(params, num)
		}
	}
	if len(params) > 0 {
		groupSize := getGroupSize(command)
		for i := 0; i < len(params); i += groupSize {
			group := params[i:min(i+groupSize, len(params))]
			processed := processGroup(group, command == strings.ToUpper(command))
			strs := make([]string, len(processed))
			for j, v := range processed {
				strs[j] = strconv.FormatFloat(v, 'f', -1, 64)
			}
			output = append(output, strings.Join(strs, " "))
		}
	}

	return strings.Join(output, " ")
}

func GenerateAllBasText(frames []v2btypes.FrameData, viewBoxW, viewBoxH int, framerate, startTime float64) []string {
	return GenerateAllBasTextWithParallel(frames, viewBoxW, viewBoxH, framerate, startTime, 4)
}

// GenerateAllBasTextWithParallel 支持并发上限
func GenerateAllBasTextWithParallel(frames []v2btypes.FrameData, viewBoxW, viewBoxH int, framerate, startTime float64, parallel int) []string {
	results := make([]string, len(frames))
	var wg sync.WaitGroup
	if parallel <= 0 {
		parallel = 1
	}
	sem := make(chan struct{}, parallel)
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.FrameData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			bas := GenerateBasText(frame, viewBoxW, viewBoxH, framerate, startTime)
			results[idx] = bas
		}(i, f)
	}
	wg.Wait()
	return results
}

// GenerateBasText 输入 FrameData 输出封装后的字符串
func GenerateBasText(frame v2btypes.FrameData, viewBoxW, viewBoxH int, framerate, startTime float64) string {
	var out strings.Builder

	for _, layer := range frame.Data {
		color := layer["color"]
		if color == "000000" {
			continue
		}
		pathData := FlipSvgPath(layer["pathdata"], viewBoxH)
		frameNum := frame.FrameIndex
		name := fmt.Sprintf("%d_%s", frameNum, color)
		displayTime := 1000.0 / framerate
		startOffset := float64(frameNum)/framerate*1000.0 - startTime

		out.WriteString(fmt.Sprintf(`
let p%s = path{d = "%s" viewBox="0 0 %d %d" width = 100%% fillColor = 0x%s alpha = 0
borderWidth = 15
    borderColor = 0x%s
}
set p%s {} %dms
then set p%s {alpha = 1} %dms
then set p%s {} %dms
then set p%s {alpha = 0} %dms
`, name, pathData, viewBoxW, viewBoxH, color, color,
			name, int(math.Floor(startOffset)),
			name, int(math.Floor(displayTime*0)),
			name, int(math.Floor(displayTime)),
			name, int(math.Floor(displayTime*0)),
		))
	}

	return out.String()
}
