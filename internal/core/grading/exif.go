package grading

import "encoding/binary"

// exifTagOrientation is the TIFF tag of the image orientation.
const exifTagOrientation = 0x0112

// jpegOrientation returns the EXIF orientation (1 to 8) of a JPEG file, or 1
// when the file has none or its EXIF block is damaged: a photo is never
// refused because of its metadata. Only the APP1 segment before the image
// data is read, and every offset is bounds-checked.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 1
		}
		marker := data[pos+1]
		if marker == 0xFF { // fill byte
			pos++
			continue
		}
		pos += 2
		switch {
		case marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			continue // no length
		case marker == 0xDA || marker == 0xD9:
			return 1 // image data or end: no EXIF before it
		}
		if pos+2 > len(data) {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[pos:]))
		if size < 2 || pos+size > len(data) {
			return 1
		}
		segment := data[pos+2 : pos+size]
		pos += size
		if marker == 0xE1 && len(segment) > 6 && string(segment[:6]) == "Exif\x00\x00" {
			return tiffOrientation(segment[6:])
		}
	}
	return 1
}

// tiffOrientation reads the orientation from the TIFF structure of an EXIF
// block (header, then the first IFD), in either byte order.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:]) != 42 {
		return 1
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[ifd:]))
	for i := range count {
		entry := ifd + 2 + i*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:]) != exifTagOrientation {
			continue
		}
		typ := order.Uint16(tiff[entry+2:])
		n := order.Uint32(tiff[entry+4:])
		if typ != 3 || n != 1 { // SHORT, one value
			return 1
		}
		if v := int(order.Uint16(tiff[entry+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}
