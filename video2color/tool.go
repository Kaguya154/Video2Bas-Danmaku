package video2color

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
	"strings"
	v2btypes "video2bas/type"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// VideoProbe 只关心视频流
type VideoProbe struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		NbFrames     string `json:"nb_frames"`      // 有些视频是字符串
		AvgFrameRate string `json:"avg_frame_rate"` // fallback
	} `json:"streams"`
}

// getTotalFrames 从 probe 数据解析总帧数
func getTotalFrames(videoPath string) (int, error) {
	probeStr, err := ffmpeg.Probe(videoPath)
	if err != nil {
		return 0, fmt.Errorf("ffprobe error: %w", err)
	}

	var probe VideoProbe
	if err := json.Unmarshal([]byte(probeStr), &probe); err != nil {
		return 0, fmt.Errorf("json unmarshal error: %w", err)
	}

	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			if stream.NbFrames != "" && stream.NbFrames != "0" {
				// nb_frames 存在则直接返回
				n, err := strconv.Atoi(stream.NbFrames)
				if err == nil {
					return n, nil
				}
			}
			// 如果 nb_frames 不存在，则使用 avg_frame_rate * duration 估算
			if stream.AvgFrameRate != "" && stream.AvgFrameRate != "0/0" {
				parts := strings.Split(stream.AvgFrameRate, "/")
				if len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den != 0 {
						return int(num / den), nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("no video stream found or cannot determine frame count")
}

// ----------------------
// 工具函数
// ----------------------

func hexToRGB(hex string) [3]int {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return [3]int{0, 0, 0}
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return [3]int{int(r), int(g), int(b)}
}

// Pixel 表示一个像素的 RGB 值

// 计算盒子范围
func calculateBoxRange(box *v2btypes.Box) {
	if len(box.Pixels) == 0 {
		return
	}

	box.RMin, box.RMax = 255, 0
	box.GMin, box.GMax = 255, 0
	box.BMin, box.BMax = 255, 0

	for _, p := range box.Pixels {
		if p.R < box.RMin {
			box.RMin = p.R
		}
		if p.R > box.RMax {
			box.RMax = p.R
		}
		if p.G < box.GMin {
			box.GMin = p.G
		}
		if p.G > box.GMax {
			box.GMax = p.G
		}
		if p.B < box.BMin {
			box.BMin = p.B
		}
		if p.B > box.BMax {
			box.BMax = p.B
		}
	}
}

// medianCutQuantize 执行中位切分颜色量化
func medianCutQuantize(img image.Image, colorCount int) []color.RGBA {
	bounds := img.Bounds()
	var pixels []v2btypes.Pixel

	// 收集所有像素
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixels = append(pixels, v2btypes.Pixel{
				R: int(r >> 8),
				G: int(g >> 8),
				B: int(b >> 8),
			})
		}
	}

	// 初始盒子
	initialBox := &v2btypes.Box{
		Pixels: pixels,
	}
	calculateBoxRange(initialBox)

	boxes := []*v2btypes.Box{initialBox}

	// 不断分割盒子
	for len(boxes) < colorCount && len(boxes) < len(pixels) {
		// 找到范围最大的盒子
		var boxToSplit *v2btypes.Box
		maxRange := -1
		for _, box := range boxes {
			rRange := box.RMax - box.RMin
			gRange := box.GMax - box.GMin
			bRange := box.BMax - box.BMin
			rangeMax := max(rRange, gRange, bRange)
			if rangeMax > maxRange {
				maxRange = rangeMax
				boxToSplit = box
			}
		}
		if boxToSplit == nil {
			break
		}

		// 选择分割通道
		rRange := boxToSplit.RMax - boxToSplit.RMin
		gRange := boxToSplit.GMax - boxToSplit.GMin
		bRange := boxToSplit.BMax - boxToSplit.BMin

		var channel string
		if rRange >= gRange && rRange >= bRange {
			channel = "R"
		} else if gRange >= rRange && gRange >= bRange {
			channel = "G"
		} else {
			channel = "B"
		}

		// 排序像素
		sort.Slice(boxToSplit.Pixels, func(i, j int) bool {
			switch channel {
			case "R":
				return boxToSplit.Pixels[i].R < boxToSplit.Pixels[j].R
			case "G":
				return boxToSplit.Pixels[i].G < boxToSplit.Pixels[j].G
			default:
				return boxToSplit.Pixels[i].B < boxToSplit.Pixels[j].B
			}
		})

		// 分成两半
		medianIndex := len(boxToSplit.Pixels) / 2
		box1 := &v2btypes.Box{Pixels: append([]v2btypes.Pixel{}, boxToSplit.Pixels[:medianIndex]...)}
		box2 := &v2btypes.Box{Pixels: append([]v2btypes.Pixel{}, boxToSplit.Pixels[medianIndex:]...)}

		calculateBoxRange(box1)
		calculateBoxRange(box2)

		// 替换盒子
		for i, b := range boxes {
			if b == boxToSplit {
				boxes = append(boxes[:i], append([]*v2btypes.Box{box1, box2}, boxes[i+1:]...)...)
				break
			}
		}
	}

	// 计算每个盒子的平均颜色
	var result []color.RGBA
	for _, box := range boxes {
		var rSum, gSum, bSum int
		for _, p := range box.Pixels {
			rSum += p.R
			gSum += p.G
			bSum += p.B
		}
		count := len(box.Pixels)
		if count == 0 {
			continue
		}
		result = append(result, color.RGBA{
			R: uint8(rSum / count),
			G: uint8(gSum / count),
			B: uint8(bSum / count),
			A: 255,
		})
	}

	return result
}
