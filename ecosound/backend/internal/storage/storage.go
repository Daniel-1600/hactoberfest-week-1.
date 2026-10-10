package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrUnsupported = errors.New("unsupported audio format")
	ErrInvalid     = errors.New("audio must decode successfully and be between 1 and 60 seconds")
	ErrTools       = errors.New("FFmpeg and FFprobe must be installed")
)

type Store struct {
	Dir, FFmpeg, FFprobe string
	DecodeTimeout        time.Duration
}
type Audio struct {
	ID, Path string
	Duration float64
}

func New(dir string) (*Store, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return nil, err
	}
	return &Store{Dir: absolute, FFmpeg: "ffmpeg", FFprobe: "ffprobe", DecodeTimeout: 30 * time.Second}, nil
}

func UUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func supportedMagic(header []byte) bool {
	if len(header) < 12 {
		return false
	}
	return (string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE") || string(header[:4]) == "fLaC" || string(header[:4]) == "OggS" || bytes.Equal(header[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) || string(header[4:8]) == "ftyp" || string(header[:3]) == "ID3" || (header[0] == 0xff && header[1]&0xe0 == 0xe0)
}

func (s *Store) Decode(ctx context.Context, reader io.Reader) (Audio, error) {
	timeout := s.DecodeTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tmp, err := os.MkdirTemp(s.Dir, ".decode-")
	if err != nil {
		return Audio{}, err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "upload")
	file, err := os.OpenFile(src, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Audio{}, err
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return Audio{}, copyErr
	}
	if closeErr != nil {
		return Audio{}, closeErr
	}
	f, err := os.Open(src)
	if err != nil {
		return Audio{}, err
	}
	header := make([]byte, 32)
	n, _ := f.Read(header)
	f.Close()
	if !supportedMagic(header[:n]) {
		return Audio{}, ErrUnsupported
	}
	if _, err = exec.LookPath(s.FFmpeg); err != nil {
		return Audio{}, ErrTools
	}
	if _, err = exec.LookPath(s.FFprobe); err != nil {
		return Audio{}, ErrTools
	}
	format, duration, err := s.probe(ctx, src)
	if err != nil {
		return Audio{}, err
	}
	accepted := map[string]bool{"wav": true, "matroska,webm": true, "mov,mp4,m4a,3gp,3g2,mj2": true, "ogg": true, "mp3": true, "flac": true}
	if !accepted[format] {
		return Audio{}, ErrUnsupported
	}
	if duration > 60.1 {
		return Audio{}, ErrInvalid
	}
	decoded := filepath.Join(tmp, "decoded.wav")
	cmd := exec.CommandContext(ctx, s.FFmpeg, "-nostdin", "-v", "error", "-protocol_whitelist", "file,pipe", "-i", src, "-map", "0:a:0", "-vn", "-ac", "1", "-ar", "32000", "-c:a", "pcm_s16le", "-t", "61", "-y", decoded)
	if err = cmd.Run(); err != nil {
		return Audio{}, fmt.Errorf("%w: decoding failed", ErrInvalid)
	}
	_, duration, err = s.probe(ctx, decoded)
	if err != nil {
		return Audio{}, err
	}
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 1 || duration > 60.1 {
		return Audio{}, ErrInvalid
	}
	id, err := UUID()
	if err != nil {
		return Audio{}, err
	}
	dest := filepath.Join(s.Dir, id+".wav")
	if err = os.Chmod(decoded, 0600); err != nil {
		return Audio{}, err
	}
	if err = os.Rename(decoded, dest); err != nil {
		return Audio{}, err
	}
	return Audio{ID: id, Path: dest, Duration: duration}, nil
}

func (s *Store) probe(ctx context.Context, path string) (string, float64, error) {
	// Restrict local protocols; an upload can never cause an HTTP/file-list fetch.
	cmd := exec.CommandContext(ctx, s.FFprobe, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=format_name,duration:stream=codec_type", "-of", "json", path)
	output, err := cmd.Output()
	if err != nil {
		return "", 0, ErrInvalid
	}
	var result struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type string `json:"codec_type"`
		} `json:"streams"`
	}
	if err = json.Unmarshal(output, &result); err != nil {
		return "", 0, ErrInvalid
	}
	hasAudio := false
	for _, stream := range result.Streams {
		if stream.Type == "audio" {
			hasAudio = true
		}
	}
	if !hasAudio {
		return "", 0, ErrInvalid
	}
	duration := 0.0
	if strings.TrimSpace(result.Format.Duration) != "" {
		duration, err = strconv.ParseFloat(result.Format.Duration, 64)
		if err != nil {
			return "", 0, ErrInvalid
		}
	}
	return result.Format.Name, duration, nil
}
