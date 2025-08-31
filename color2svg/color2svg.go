package color2svg

import (
	"bytes"
	"image"
	v2btypes "src/type"

	"github.com/gotranspile/gotrace"
)

// ConvertToSVG 使用 gotrace 将 FrameLayers 转成 SVG
func ConvertToSVG(frames []v2btypes.FrameLayers) ([]v2btypes.FrameSVG, error) {
	result := make([]v2btypes.FrameSVG, len(frames))

	for fi, frame := range frames {
		fsvg := v2btypes.FrameSVG{
			FrameIndex: frame.Index,
			Layers:     make([]v2btypes.LayerSVG, len(frame.Layers)),
		}

		for li, layer := range frame.Layers {
			svgStr, err := traceGrayToSVG(layer.Mask)
			if err != nil {
				return nil, err
			}
			fsvg.Layers[li] = v2btypes.LayerSVG{
				ColorIndex: li,
				SVGData:    svgStr,
			}
		}
		result[fi] = fsvg
	}

	return result, nil
}

// traceGrayToSVG 核心：使用 gotrace 将 image.Gray 转 SVG 字符串
func traceGrayToSVG(mask *image.Gray) (string, error) {
	bm := gotrace.BitmapFromGray(mask, nil)

	paths, err := gotrace.Trace(bm, nil)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	sz := mask.Bounds().Size()
	if err := gotrace.Render("svg", nil, &buf, paths, sz.X, sz.Y); err != nil {
		return "", err
	}

	return buf.String(), nil
}
