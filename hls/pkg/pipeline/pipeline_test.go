package pipeline

import (
	"strings"
	"testing"
)

func TestBuildFFmpegArgsCPU(t *testing.T) {
	opts := Options{
		Width:        1920,
		Height:       1080,
		Framerate:    30,
		VideoBitrate: 6000,
		Encoder:      "cpu",
		HLSTime:      2,
		HLSListSize:  5,
		AudioSource:  "stream_sink.monitor",
		HLSDir:       "/tmp/test_hls",
	}

	args := BuildFFmpegArgs(opts)
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "-s 1920x1080") {
		t.Errorf("expected resolution 1920x1080 in ffmpeg args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-c:v libx264") {
		t.Errorf("expected -c:v libx264 in cpu ffmpeg args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-r 30") {
		t.Errorf("expected framerate 30, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-b:v 6000k") {
		t.Errorf("expected video bitrate 6000k, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-i stream_sink.monitor") {
		t.Errorf("expected audio source stream_sink.monitor, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-hls_time 2") {
		t.Errorf("expected hls_time 2, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "/tmp/test_hls/index.m3u8") {
		t.Errorf("expected manifest path in args, got: %s", cmdStr)
	}
}

func TestBuildFFmpegArgsGPU(t *testing.T) {
	opts := Options{
		Width:        1920,
		Height:       1080,
		Framerate:    30,
		VideoBitrate: 6000,
		Encoder:      "gpu",
		HLSTime:      2,
		HLSListSize:  5,
		AudioSource:  "stream_sink.monitor",
		HLSDir:       "/tmp/test_hls",
	}

	args := BuildFFmpegArgs(opts)
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "-c:v h264_vaapi") {
		t.Errorf("expected -c:v h264_vaapi in gpu ffmpeg args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-vaapi_device /dev/dri/renderD128") {
		t.Errorf("expected vaapi_device in gpu ffmpeg args, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-b:v 6000k") {
		t.Errorf("expected video bitrate 6000k, got: %s", cmdStr)
	}
}

func TestBuildFFmpegArgsNVENC(t *testing.T) {
	opts := Options{
		Width:        1920,
		Height:       1080,
		Framerate:    60,
		VideoBitrate: 8000,
		Encoder:      "nvenc",
		HLSTime:      2,
		HLSListSize:  5,
		AudioSource:  "stream_sink.monitor",
		HLSDir:       "/tmp/test_hls",
	}

	args := BuildFFmpegArgs(opts)
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "-c:v h264_nvenc") {
		t.Errorf("expected -c:v h264_nvenc in nvenc ffmpeg args, got: %s", cmdStr)
	}
}

func TestBuildFFmpegArgsNoAudio(t *testing.T) {
	opts := Options{
		Width:        3840,
		Height:       2160,
		Framerate:    60,
		VideoBitrate: 8000,
		HLSTime:      1,
		HLSListSize:  10,
		AudioSource:  "none",
		HLSDir:       "/tmp/test_hls",
	}

	args := BuildFFmpegArgs(opts)
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "-an") {
		t.Errorf("expected -an flag when audio is none, got: %s", cmdStr)
	}
}

func TestBuildGstPipelineStr(t *testing.T) {
	pipelineStr := BuildGstPipelineStr(1920, 1080, 30)

	if !strings.Contains(pipelineStr, "pipewiresrc") {
		t.Errorf("expected pipewiresrc in pipeline, got: %s", pipelineStr)
	}
	if !strings.Contains(pipelineStr, "videoscale") {
		t.Errorf("expected videoscale in pipeline, got: %s", pipelineStr)
	}
	if !strings.Contains(pipelineStr, "videorate") {
		t.Errorf("expected videorate in pipeline, got: %s", pipelineStr)
	}
	if !strings.Contains(pipelineStr, "width=1920,height=1080,framerate=30/1") {
		t.Errorf("expected caps filter in pipeline, got: %s", pipelineStr)
	}
}
