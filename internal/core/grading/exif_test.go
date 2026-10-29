package grading

import (
	"encoding/binary"
	"testing"
)

func TestJPEGOrientation(t *testing.T) {
	soi := []byte{0xFF, 0xD8}
	sos := []byte{0xFF, 0xDA, 0x00, 0x02}
	join := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	exif := exifSegment

	// A TIFF block with the orientation entry changed by edit.
	damaged := func(edit func(seg []byte)) []byte {
		seg := exif(binary.LittleEndian, 6)
		edit(seg)
		return join(soi, seg, sos)
	}
	// Offsets in the segment: 4 marker and length, 6 "Exif\0\0", then the TIFF
	// block: 0 order, 2 magic, 4 IFD offset, 8 count, 10 entry (tag 10, type
	// 12, count 14, value 18).
	const tiff = 4 + 6

	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"little endian 6", join(soi, exif(binary.LittleEndian, 6), sos), 6},
		{"big endian 8", join(soi, exif(binary.BigEndian, 8), sos), 8},
		{"orientation 1", join(soi, exif(binary.BigEndian, 1), sos), 1},
		{"after a JFIF segment", join(soi, appSegment(0xE0, []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")), exif(binary.LittleEndian, 3), sos), 3},
		{"after an XMP segment", join(soi, appSegment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x/>")), exif(binary.BigEndian, 5), sos), 5},
		{"after fill bytes", join(soi, []byte{0xFF, 0xFF}, exif(binary.BigEndian, 7), sos), 7},
		{"after a marker without length", join(soi, []byte{0xFF, 0xD0}, exif(binary.BigEndian, 2), sos), 2},
		{"exif after the image data is not looked for", join(soi, sos, exif(binary.BigEndian, 6)), 1},
		{"no exif", join(soi, sos), 1},
		{"empty", nil, 1},
		{"not a jpeg", []byte("PNG..."), 1},
		{"only soi", soi, 1},
		{"byte that is not a marker", join(soi, []byte{0x00, 0x00, 0x00, 0x00, 0x00}), 1},
		{"segment longer than the file", join(soi, []byte{0xFF, 0xE1, 0x7F, 0xFF, 'E', 'x'}), 1},
		{"segment length below 2", join(soi, []byte{0xFF, 0xE1, 0x00, 0x01, 0, 0}), 1},
		{"segment length missing", join(soi, []byte{0xFF, 0xE1, 0xFF}), 1},
		{"bad byte order", damaged(func(s []byte) { s[tiff] = 'X' }), 1},
		{"bad magic", damaged(func(s []byte) { s[tiff+2] = 43 }), 1},
		{"ifd offset outside", damaged(func(s []byte) { s[tiff+4] = 200 }), 1},
		{"ifd offset inside the header", damaged(func(s []byte) { s[tiff+4] = 2 }), 1},
		{"entry count beyond the data", damaged(func(s []byte) { s[tiff+8] = 50; s[tiff+10] = 0x0F }), 1},
		{"other tag only", damaged(func(s []byte) { s[tiff+10] = 0x0F }), 1},
		{"wrong type", damaged(func(s []byte) { s[tiff+12] = 4 }), 1},
		{"wrong count", damaged(func(s []byte) { s[tiff+14] = 2 }), 1},
		{"orientation 0", damaged(func(s []byte) { s[tiff+18] = 0 }), 1},
		{"orientation 9", damaged(func(s []byte) { s[tiff+18] = 9 }), 1},
		{"orientation 65535", damaged(func(s []byte) { s[tiff+18], s[tiff+19] = 0xFF, 0xFF }), 1},
		{"exif block too short", join(soi, appSegment(0xE1, []byte("Exif\x00\x00II*\x00")), sos), 1},
		{"exif header only", join(soi, appSegment(0xE1, []byte("Exif\x00\x00")), sos), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jpegOrientation(tt.data); got != tt.want {
				t.Errorf("jpegOrientation = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJPEGOrientationSecondEntry(t *testing.T) {
	// An IFD with two entries where the orientation is the second.
	var tiff []byte
	tiff = append(tiff, "MM"...)
	tiff = binary.BigEndian.AppendUint16(tiff, 42)
	tiff = binary.BigEndian.AppendUint32(tiff, 8)
	tiff = binary.BigEndian.AppendUint16(tiff, 2)
	// entry 1: tag 0x010F (make), type 2 (ASCII), count 1, value
	tiff = binary.BigEndian.AppendUint16(tiff, 0x010F)
	tiff = binary.BigEndian.AppendUint16(tiff, 2)
	tiff = binary.BigEndian.AppendUint32(tiff, 1)
	tiff = append(tiff, 'A', 0, 0, 0)
	// entry 2: orientation 6
	tiff = binary.BigEndian.AppendUint16(tiff, exifTagOrientation)
	tiff = binary.BigEndian.AppendUint16(tiff, 3)
	tiff = binary.BigEndian.AppendUint32(tiff, 1)
	tiff = binary.BigEndian.AppendUint16(tiff, 6)
	tiff = append(tiff, 0, 0)
	data := append([]byte{0xFF, 0xD8}, appSegment(0xE1, append([]byte("Exif\x00\x00"), tiff...))...)
	data = append(data, 0xFF, 0xDA, 0, 2)
	if got := jpegOrientation(data); got != 6 {
		t.Errorf("jpegOrientation = %d, want 6", got)
	}
}
