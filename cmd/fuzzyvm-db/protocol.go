package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Wire protocol between the generate server (which owns the pebble database)
// and the fuzz-worker clients. It runs over a Unix domain socket.
//
// Every message is: 1-byte opcode, then a 4-byte big-endian frame length, then
// that many payload bytes.
//
//	HAS: payload is a 32-byte code hash. The server replies with a single byte,
//	     respPresent or respAbsent.
//	PUT: payload is one or more code blobs, each a 4-byte big-endian length
//	     followed by the bytes. The server stores any it doesn't already have
//	     and replies with a single respAck byte (so the worker sees write errors
//	     and gets natural backpressure).
const (
	opHas byte = 'H'
	opPut byte = 'P'

	respAbsent  byte = 0
	respPresent byte = 1
	respAck     byte = 2

	// maxFrame bounds a single frame to guard against a corrupt length header
	// turning into a huge allocation. Programs are capped at ~10KB by the
	// generator, plus framing overhead; 1 MiB is comfortably above that.
	maxFrame = 1 << 20
)

// writeFrame writes opcode + length-prefixed payload.
func writeFrame(w io.Writer, op byte, payload []byte) error {
	if len(payload) > maxFrame {
		return fmt.Errorf("frame too large: %d", len(payload))
	}
	var hdr [5]byte
	hdr[0] = op
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readByte reads a single response byte (the server's reply to HAS/PUT).
func readByte(r io.Reader) (byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}

// readFrame reads an opcode + length-prefixed payload.
func readFrame(r io.Reader) (op byte, payload []byte, err error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(hdr[1:])
	if n > maxFrame {
		return 0, nil, fmt.Errorf("frame too large: %d", n)
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return hdr[0], payload, nil
}

// encodeBlobs concatenates code blobs as length-prefixed chunks for a PUT.
func encodeBlobs(blobs ...[]byte) []byte {
	size := 0
	for _, b := range blobs {
		size += 4 + len(b)
	}
	out := make([]byte, 0, size)
	var lenBuf [4]byte
	for _, b := range blobs {
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(b)))
		out = append(out, lenBuf[:]...)
		out = append(out, b...)
	}
	return out
}

// decodeBlobs splits a PUT payload back into individual code blobs.
func decodeBlobs(payload []byte) ([][]byte, error) {
	var blobs [][]byte
	for len(payload) > 0 {
		if len(payload) < 4 {
			return nil, fmt.Errorf("truncated blob length")
		}
		n := binary.BigEndian.Uint32(payload[:4])
		payload = payload[4:]
		if uint32(len(payload)) < n {
			return nil, fmt.Errorf("truncated blob body: want %d have %d", n, len(payload))
		}
		blobs = append(blobs, payload[:n])
		payload = payload[n:]
	}
	return blobs, nil
}
