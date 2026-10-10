package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func pcmWAV(seconds int) []byte {
	samples := 32000 * seconds
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+samples*2))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(32000))
	binary.Write(&b, binary.LittleEndian, uint32(64000))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(samples*2))
	b.Write(make([]byte, samples*2))
	return b.Bytes()
}

func TestDecodeAndCleanup(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"text", []byte("this is not audio"), ErrUnsupported},
		{"corrupt WAV", []byte("RIFF1111WAVEinvalid payload"), ErrInvalid},
		{"zero frames", pcmWAV(0), ErrInvalid},
		{"too long", pcmWAV(61), ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Decode(context.Background(), bytes.NewReader(tc.data))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			entries, _ := os.ReadDir(s.Dir)
			if len(entries) != 0 {
				t.Fatalf("left temp files: %v", entries)
			}
		})
	}
	audio, err := s.Decode(context.Background(), bytes.NewReader(pcmWAV(3)))
	if err != nil {
		t.Fatal(err)
	}
	if audio.Duration != 3 || filepath.Base(audio.Path) != audio.ID+".wav" {
		t.Fatalf("bad decoded audio: %+v", audio)
	}
	entries, _ := os.ReadDir(s.Dir)
	if len(entries) != 1 {
		t.Fatalf("left temporary files: %v", entries)
	}
	if _, err := os.Stat(audio.Path); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserContainersConvertToWAV(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.wav")
	if err := os.WriteFile(source, pcmWAV(3), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ extension, codec string }{{"webm", "libopus"}, {"m4a", "aac"}, {"ogg", "libopus"}, {"mp3", "libmp3lame"}, {"flac", "flac"}} {
		t.Run(tc.extension, func(t *testing.T) {
			encoded := filepath.Join(tmp, "browser."+tc.extension)
			output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-i", source, "-c:a", tc.codec, "-y", encoded).CombinedOutput()
			if err != nil {
				t.Fatalf("encode fixture: %s %v", output, err)
			}
			s, err := New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(encoded)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			audio, err := s.Decode(context.Background(), file)
			if err != nil {
				t.Fatal(err)
			}
			if audio.Duration < 2.9 || audio.Duration > 3.2 {
				t.Fatalf("unexpected duration %v", audio.Duration)
			}
			data, err := os.ReadFile(audio.Path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
				t.Fatal("output is not WAV")
			}
		})
	}
}
