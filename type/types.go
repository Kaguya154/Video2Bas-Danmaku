package v2btypes

import (
	"image"
	"image/color"
)

// LayerSVG 表示单个颜色图层的 SVG
type LayerSVG struct {
	ColorIndex int
	SVGData    string
}

// FrameSVG 表示一帧所有颜色层的 SVG
type FrameSVG struct {
	FrameIndex int
	Layers     []LayerSVG
}

// FrameData 封装输出的数据结构
type FrameData struct {
	FrameIndex int                 `json:"frameIndex"`
	Data       []map[string]string `json:"data"`
}

// Frame 表示一帧图像
type Frame struct {
	Index int
	Image image.Image
}

// ColorLayer 表示某一帧中某个颜色的分割图层
type ColorLayer struct {
	Color color.RGBA  // 颜色 HEX（如 "FF0000"）
	Mask  *image.Gray // 黑白掩码图：黑=该颜色，白=其他
}

// FrameLayers 表示某一帧的分层结果
type FrameLayers struct {
	Index  int
	Layers []ColorLayer
}

type Pixel struct {
	R, G, B int
}

// Box 表示颜色盒子
type Box struct {
	Pixels     []Pixel
	RMin, RMax int
	GMin, GMax int
	BMin, BMax int
}
