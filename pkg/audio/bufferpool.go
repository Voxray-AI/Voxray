package audio

import "sync"

// Tiered audio buffer pools for common sizes.
// Sizes are chosen for typical frame/chunk sizes:
// - 160 bytes  — 10ms @ 8kHz 16‑bit mono
// - 320 bytes  — 10ms @ 16kHz 16‑bit mono (common STT input)
// - 960 bytes  — 20ms @ 48kHz (Opus frame)
// - 4096 bytes — larger WebSocket/audio messages

var audioBufferPools = [...]struct {
	size int
	pool *sync.Pool
}{
	{size: 160, pool: &sync.Pool{New: func() any { return make([]byte, 160) }}},
	{size: 320, pool: &sync.Pool{New: func() any { return make([]byte, 320) }}},
	{size: 960, pool: &sync.Pool{New: func() any { return make([]byte, 960) }}},
	{size: 4096, pool: &sync.Pool{New: func() any { return make([]byte, 4096) }}},
}

// GetAudioBuffer returns a byte slice with length 0 and capacity >= size.
// It prefers the smallest pool whose size is >= requested size; if none match,
// it allocates a new buffer of the requested size directly.
func GetAudioBuffer(size int) []byte {
	if size <= 0 {
		size = audioBufferPools[0].size
	}
	for i := range audioBufferPools {
		if size <= audioBufferPools[i].size {
			buf := audioBufferPools[i].pool.Get().([]byte)
			return buf[:0]
		}
	}
	return make([]byte, 0, size)
}

// PutAudioBuffer returns a buffer to the appropriate pool based on its capacity.
// Buffers that do not match any known tier are dropped.
func PutAudioBuffer(buf []byte) {
	capacity := cap(buf)
	for i := range audioBufferPools {
		if capacity == audioBufferPools[i].size {
			audioBufferPools[i].pool.Put(buf[:audioBufferPools[i].size])
			return
		}
	}
}

