package main

import (
	"sync"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/nic/devtoolkit/internal/pkg/mediax"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// MediaService exposes batch HLS (m3u8) to MP4/other-container conversion (R19).
// Conversion is delegated to a locally installed ffmpeg binary; each source may
// be a remote URL or a local playlist file, and every output is written into a
// chosen folder keeping the source's base name with the target extension.
type MediaService struct {
	batchMu   sync.Mutex
	batchJobs map[string]*mediax.BatchJob
}

// CheckFFmpeg reports whether ffmpeg is available so the UI can guide install.
func (s *MediaService) CheckFFmpeg() mediax.FFmpegInfo {
	return mediax.CheckFFmpeg()
}

// ProbeM3U8 parses the playlist and returns its duration, segment count and
// encryption/live flags so the user knows what they're about to convert.
func (s *MediaService) ProbeM3U8(source string) (mediax.ProbeResult, error) {
	return mediax.Probe(source)
}

// PickInputFiles opens a native dialog to choose one or more local .m3u8
// playlists and returns their paths (empty when the user cancels).
func (s *MediaService) PickInputFiles() ([]string, error) {
	dialog := application.Get().Dialog.OpenFile()
	dialog.SetTitle("选择 m3u8 播放列表（可多选）")
	dialog.CanChooseFiles(true)
	dialog.AddFilter("m3u8 播放列表", "*.m3u8;*.m3u")
	paths, err := dialog.PromptForMultipleSelection()
	if err != nil {
		return nil, apperr.Newf(apperr.InvalidInput, "无法打开文件选择框：%v", err)
	}
	return paths, nil
}

// PickOutputDir opens a native dialog to choose the folder batch results are
// written to, returning its path (empty when the user cancels).
func (s *MediaService) PickOutputDir() (string, error) {
	dialog := application.Get().Dialog.OpenFile()
	dialog.SetTitle("选择输出文件夹")
	dialog.CanChooseFiles(false)
	dialog.CanChooseDirectories(true)
	dialog.CanCreateDirectories(true)
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", apperr.Newf(apperr.InvalidInput, "无法打开文件夹选择框：%v", err)
	}
	return path, nil
}

// StartBatchConvert converts several m3u8 sources into outputDir, up to
// concurrency at a time. Each output reuses its source's base name with the
// target format's extension. It returns a job id to poll with BatchProgress.
func (s *MediaService) StartBatchConvert(sources []string, outputDir string, opts mediax.Options, concurrency int) (string, error) {
	job, err := mediax.StartBatch(sources, outputDir, opts, concurrency)
	if err != nil {
		return "", err
	}
	id, err := newJobID()
	if err != nil {
		job.Cancel()
		return "", err
	}
	s.batchMu.Lock()
	if s.batchJobs == nil {
		s.batchJobs = map[string]*mediax.BatchJob{}
	}
	s.batchJobs[id] = job
	s.batchMu.Unlock()
	return id, nil
}

// BatchProgress returns the latest snapshot for a batch conversion job.
func (s *MediaService) BatchProgress(id string) (mediax.BatchProgress, error) {
	s.batchMu.Lock()
	job := s.batchJobs[id]
	s.batchMu.Unlock()
	if job == nil {
		return mediax.BatchProgress{}, apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	return job.Snapshot(), nil
}

// CancelBatchConvert stops a running batch conversion and forgets it.
func (s *MediaService) CancelBatchConvert(id string) error {
	s.batchMu.Lock()
	job := s.batchJobs[id]
	delete(s.batchJobs, id)
	s.batchMu.Unlock()
	if job == nil {
		return apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	job.Cancel()
	return nil
}
