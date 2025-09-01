# Video2Bas Danmaku 视频转换Bas弹幕

## Requirement 系统需求

- Install `ffmpeg` 安装`ffmpeg`


## Usage 使用

```shell
Usage of video2bas:
  -colors int
        颜色数量 (default 4)
  -fps int
        每秒帧数 (default 10)
  -help
        显示帮助信息
  -maxsize int
        单个输出文件最大尺寸，单位字节 (default 2097152)
  -output string
        输出文件路径 (default "output/video")
  -parallel int
        并行处理的最大协程数 (default 4)
  -serial
        是否串行处理以最大程度减少内存使用
  -viedo string
        视频文件路径
  -width int
        最大宽度 (default 96
```

Example: 示例：
```shell
.\video2bas-windows-amd64.exe -viedo "badapple.mp4" -fps 30 -width 540 -serial true
```