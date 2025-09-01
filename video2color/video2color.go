package video2color

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	v2btypes "video2bas/type"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func ExtractFrames(ctx context.Context, videoPath string, fps, maxWidth int) ([]v2btypes.Frame, error) {
	if fps <= 0 {
		fps = 1
	}
	r, w := io.Pipe()

	go func() {
		defer w.Close()
		cmd := ffmpeg.Input(videoPath).
			Output("pipe:1", ffmpeg.KwArgs{
				"format":   "image2pipe",
				"vcodec":   "png",
				"r":        strconv.Itoa(fps),
				"vf":       fmt.Sprintf("scale=%d:-1", maxWidth),
				"loglevel": "error",
			}).
			WithOutput(w).
			WithErrorOutput(os.Stderr)

		err := cmd.Run()
		if err != nil {
			w.CloseWithError(fmt.Errorf("ffmpeg error: %w", err))
			return
		}
	}()

	var frames []v2btypes.Frame
	reader := bufio.NewReader(r)
	index := 0

	for {
		img, err := png.Decode(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break // 正常结束
			}
			if strings.Contains(err.Error(), "unexpected EOF") {
				break // 管道最后一帧解码完后会出现 EOF，直接退出
			}
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

// ExtractFramesStream 返回 bufio.Reader，调用方可流式读取帧
func ExtractFramesStream(ctx context.Context, videoPath string, fps, maxWidth int) (*bufio.Reader, io.Closer, error) {
	if fps <= 0 {
		fps = 1
	}

	r, w := io.Pipe()

	go func() {
		defer w.Close()
		cmd := ffmpeg.Input(videoPath).
			Output("pipe:1", ffmpeg.KwArgs{
				"format":   "image2pipe",
				"vcodec":   "png",
				"r":        strconv.Itoa(fps),
				"vf":       fmt.Sprintf("scale=%d:-1", maxWidth),
				"loglevel": "error",
			}).
			WithOutput(w).
			WithErrorOutput(os.Stderr)

		err := cmd.Run()
		if err != nil {
			w.CloseWithError(fmt.Errorf("ffmpeg error: %w", err))
			return
		}
	}()

	return bufio.NewReader(r), r, nil
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
func SplitAllFrames(frames []v2btypes.Frame, rgb []color.RGBA, parallel int) ([]v2btypes.FrameLayers, error) {
	if len(frames) == 0 {
		return nil, errors.New("no frames provided")
	}
	if len(rgb) == 0 {
		return nil, errors.New("empty palette")
	}
	if parallel <= 0 {
		parallel = 1
	}

	results := make([]v2btypes.FrameLayers, len(frames))
	errs := make(chan error, len(frames))
	sem := make(chan struct{}, parallel)

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.Frame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
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

// SplitAllFramesAuto 对多帧进行颜色分层（并行版，带并发上限）
func SplitAllFramesAuto(frames []v2btypes.Frame, colorCount int, parallel int) ([]v2btypes.FrameLayers, error) {
	if len(frames) == 0 {
		return nil, errors.New("no frames provided")
	}
	if parallel <= 0 {
		parallel = 1
	}

	results := make([]v2btypes.FrameLayers, len(frames))
	errs := make(chan error, len(frames))
	sem := make(chan struct{}, parallel)

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.Frame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
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

// SplitAllFramesAutoWithProgress 支持进度回调
func SplitAllFramesAutoWithProgress(frames []v2btypes.Frame, colorCount int, parallel int, progress func()) ([]v2btypes.FrameLayers, error) {
	if len(frames) == 0 {
		return nil, errors.New("no frames provided")
	}
	if parallel <= 0 {
		parallel = 1
	}

	results := make([]v2btypes.FrameLayers, len(frames))
	errs := make(chan error, len(frames))
	sem := make(chan struct{}, parallel)

	var wg sync.WaitGroup
	for i, f := range frames {
		wg.Add(1)
		go func(idx int, frame v2btypes.Frame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			layers, err := SplitColorsAuto(frame, colorCount)
			if err != nil {
				errs <- err
				return
			}
			results[idx] = layers
			if progress != nil {
				progress()
			}
		}(i, f)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return nil, err
	}
	return results, nil
}
