// Package originalgraphics preserves the original planar graphics as indices.
package originalgraphics

import (
	"encoding/binary"
	"fmt"
	"image/color"
)

type Indexed struct {
	Width, Height int
	Pixels        []byte
	Palette       []color.NRGBA
	Transparent   int
}

// DAC expands the high storage nibble to the actual Amiga twelve-bit output.
func DAC(value uint8) uint8 { return (value >> 4) * 17 }

// DecodeIFF reads the original ILBM planes and ByteRun1 rows without merging
// distinct palette indices that happen to have the same RGB values.
func DecodeIFF(data []byte) (Indexed, error) {
	result := Indexed{Transparent: -1}
	if len(data) < 12 || string(data[:4]) != "FORM" || string(data[8:12]) != "ILBM" {
		return result, fmt.Errorf("invalid original ILBM")
	}
	var header, body []byte
	for at := 12; at+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[at+4:]))
		if n > len(data)-at-8 {
			return result, fmt.Errorf("truncated original ILBM chunk")
		}
		chunk := data[at+8 : at+8+n]
		switch string(data[at : at+4]) {
		case "BMHD":
			header = chunk
		case "CMAP":
			for index := 0; index+2 < n; index += 3 {
				result.Palette = append(result.Palette, color.NRGBA{DAC(chunk[index]), DAC(chunk[index+1]), DAC(chunk[index+2]), 255})
			}
		case "BODY":
			body = chunk
		}
		at += 8 + n + n%2
	}
	if len(header) < 20 {
		return result, fmt.Errorf("missing original ILBM header")
	}
	result.Width, result.Height = int(binary.BigEndian.Uint16(header)), int(binary.BigEndian.Uint16(header[2:]))
	planes, masking, compression := int(header[8]), header[9], header[10]
	if planes < 1 || planes > 8 || len(result.Palette) < 1<<planes || result.Width*result.Height > 1024*1024 || compression > 1 {
		return result, fmt.Errorf("unsupported original ILBM geometry or compression")
	}
	if masking == 2 {
		result.Transparent = int(binary.BigEndian.Uint16(header[12:]))
	}
	rowBytes, stored := (result.Width+15)/16*2, planes
	if masking == 1 {
		stored++
	}
	result.Pixels = make([]byte, result.Width*result.Height)
	cursor := 0
	for y := 0; y < result.Height; y++ {
		rows := make([][]byte, stored)
		for plane := range rows {
			row := make([]byte, 0, rowBytes)
			for len(row) < rowBytes {
				if cursor >= len(body) {
					return result, fmt.Errorf("truncated original ILBM body")
				}
				if compression == 0 {
					if cursor+rowBytes > len(body) {
						return result, fmt.Errorf("truncated original ILBM row")
					}
					row = append(row, body[cursor:cursor+rowBytes]...)
					cursor += rowBytes
					break
				}
				control := int(int8(body[cursor]))
				cursor++
				if control == -128 {
					continue
				}
				n := control + 1
				if control < 0 {
					n = 1 - control
				}
				if len(row)+n > rowBytes || cursor >= len(body) || control >= 0 && cursor+n > len(body) {
					return result, fmt.Errorf("invalid original ILBM run")
				}
				if control >= 0 {
					row = append(row, body[cursor:cursor+n]...)
					cursor += n
				} else {
					for index := 0; index < n; index++ {
						row = append(row, body[cursor])
					}
					cursor++
				}
			}
			rows[plane] = row
		}
		for x := 0; x < result.Width; x++ {
			var index byte
			for plane := 0; plane < planes; plane++ {
				if rows[plane][x/8]&(0x80>>uint(x%8)) != 0 {
					index |= 1 << plane
				}
			}
			result.Pixels[y*result.Width+x] = index
		}
	}
	return result, nil
}
