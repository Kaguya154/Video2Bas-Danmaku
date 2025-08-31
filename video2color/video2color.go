package video2color

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"sort"
	v2btypes "src/type"
	"strconv"
	"strings"
	"sync"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func ExtractFrames(ctx context.Context, videoPath string, fps, maxWidth int) ([]v2btypes.Frame, error) {
	if fps <= 0 {
		fps = 1
	}

	r, w := io.Pipe()

	cmd := ffmpeg.Input(videoPath).
		Output("pipe:1", ffmpeg.KwArgs{
			"format": "image2pipe",
			"vcodec": "png",
			"r":      strconv.Itoa(fps),
			"vf":     fmt.Sprintf("scale=%d:-1", maxWidth),
		}).
		WithOutput(w).
		WithErrorOutput(os.Stderr)
	cmd.Context = ctx

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	//从reader读取512字节，打印为HEX
	var buf [512]byte
	n, _ := r.Read(buf[:])
	fmt.Printf("read %d bytes: %x\n", n, buf[:n])

	var frames []v2btypes.Frame
	reader := bufio.NewReader(r)
	index := 0

	for {
		img, _, err := image.Decode(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {

			return nil, fmt.Errorf("decode frame %d failed: %w", index, err)
		}
		frames = append(frames, v2btypes.Frame{Index: index, Image: img})
		index++
	}

	if len(frames) == 0 {
		return nil, errors.New("no frames extracted")
	}

	return frames, nil
}

func SplitColorsAuto(frame v2btypes.Frame, colorCount int) (v2btypes.FrameLayers, error) {
	quantize := medianCutQuantize(frame.Image, colorCount)
	return SplitColors(frame, quantize)
}

// SplitColors 将一帧拆分为颜色图层
func SplitColors(frame v2btypes.Frame, rgb []color.RGBA) (v2btypes.FrameLayers, error) {
	if frame.Image == nil {
		return v2btypes.FrameLayers{}, errors.New("nil image")
	}
	if len(rgb) == 0 {
		return v2btypes.FrameLayers{}, errors.New("empty palette")
	}

	bounds := frame.Image.Bounds()
	layers := make([]v2btypes.ColorLayer, len(rgb))
	for i, hex := range rgb {
		layers[i] = v2btypes.ColorLayer{
			Color: hex,
			Mask:  image.NewGray(bounds),
		}
		// 默认白色背景
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				layers[i].Mask.Set(x, y, color.Gray{Y: 255})
			}
		}
	}

	// 遍历像素
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := frame.Image.At(x, y).RGBA()
			rr, gg, bb := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			// 找最近颜色
			bestIdx := 0
			bestDist := math.MaxFloat64
			for i, rgb := range rgb {
				dr := float64(rr - rgb.R)
				dg := float64(gg - rgb.G)
				db := float64(bb - rgb.B)
				dist := dr*dr + dg*dg + db*db
				if dist < bestDist {
					bestDist = dist
					bestIdx = i
				}
			}
			// 在目标图层上标记黑色
			layers[bestIdx].Mask.Set(x, y, color.Gray{Y: 0})
		}
	}

	return v2btypes.FrameLayers{Index: frame.Index, Layers: layers}, nil
}

// SplitAllFrames 对多帧进行颜色分层（并行版）
func SplitAllFramesAuto(frames []v2btypes.Frame, colorCount int) ([]v2btypes.FrameLayers, error) {
	if len(frames) == 0 {
		return nil, errors.New("no frames provided")
	}

	results := make([]v2btypes.FrameLayers, len(frames))
	errs := make(chan error, len(frames))

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.Frame) {
			defer wg.Done()
			layers, err := SplitColorsAuto(frame, colorCount)
			if err != nil {
				errs <- err
				return
			}
			results[idx] = layers
		}(i, f)
	}

	wg.Wait()
	close(errs)

	// 返回第一个错误（如果有）
	for err := range errs {
		return nil, err
	}
	return results, nil
}

// SplitAllFrames 对多帧进行颜色分层（并行版）
func SplitAllFrames(frames []v2btypes.Frame, rgb []color.RGBA) ([]v2btypes.FrameLayers, error) {
	if len(frames) == 0 {
		return nil, errors.New("no frames provided")
	}
	if len(rgb) == 0 {
		return nil, errors.New("empty palette")
	}

	results := make([]v2btypes.FrameLayers, len(frames))
	errs := make(chan error, len(frames))

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.Frame) {
			defer wg.Done()
			layers, err := SplitColors(frame, rgb)
			if err != nil {
				errs <- err
				return
			}
			results[idx] = layers
		}(i, f)
	}

	wg.Wait()
	close(errs)

	// 返回第一个错误（如果有）
	for err := range errs {
		return nil, err
	}
	return results, nil
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
