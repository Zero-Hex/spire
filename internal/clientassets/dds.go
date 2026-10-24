package clientassets

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
)

// decodeDDS reads the top mip level of a DDS texture: DXT1, DXT3, DXT5, or
// uncompressed 24/32-bit. That covers the client's icon sheets (DXT5) and
// what the weapon toolkit writes. DX10 headers are not supported.
func decodeDDS(data []byte) (*image.NRGBA, error) {
	if len(data) < 128 || string(data[0:4]) != "DDS " {
		return nil, errors.New("not a DDS file")
	}

	height := int(binary.LittleEndian.Uint32(data[12:16]))
	width := int(binary.LittleEndian.Uint32(data[16:20]))
	pfFlags := binary.LittleEndian.Uint32(data[80:84])
	fourCC := string(data[84:88])
	bitCount := int(binary.LittleEndian.Uint32(data[88:92]))
	masks := [4]uint32{
		binary.LittleEndian.Uint32(data[92:96]),
		binary.LittleEndian.Uint32(data[96:100]),
		binary.LittleEndian.Uint32(data[100:104]),
		binary.LittleEndian.Uint32(data[104:108]),
	}
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return nil, fmt.Errorf("bad DDS size %vx%v", width, height)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	body := data[128:]

	const ddpfFourCC = 0x4
	if pfFlags&ddpfFourCC != 0 {
		blockSize := 16
		if fourCC == "DXT1" {
			blockSize = 8
		} else if fourCC != "DXT3" && fourCC != "DXT5" {
			return nil, fmt.Errorf("unsupported DDS format %q", fourCC)
		}

		bw, bh := (width+3)/4, (height+3)/4
		if len(body) < bw*bh*blockSize {
			return nil, errors.New("DDS data is truncated")
		}

		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				block := body[(by*bw+bx)*blockSize:]
				var pixels [16]color.NRGBA
				switch fourCC {
				case "DXT1":
					decodeColorBlock(block[0:8], &pixels, true)
				case "DXT3":
					decodeColorBlock(block[8:16], &pixels, false)
					for i := 0; i < 16; i++ {
						a := (block[i/2] >> (uint(i%2) * 4)) & 0xF
						pixels[i].A = a * 17
					}
				case "DXT5":
					decodeColorBlock(block[8:16], &pixels, false)
					decodeAlphaBlock(block[0:8], &pixels)
				}
				for i := 0; i < 16; i++ {
					x, y := bx*4+i%4, by*4+i/4
					if x < width && y < height {
						img.SetNRGBA(x, y, pixels[i])
					}
				}
			}
		}
		return img, nil
	}

	// uncompressed: channels by bit mask
	if bitCount != 24 && bitCount != 32 {
		return nil, fmt.Errorf("unsupported DDS bit count %v", bitCount)
	}
	const ddpfAlphaPixels = 0x1
	bpp := bitCount / 8
	if len(body) < width*height*bpp {
		return nil, errors.New("DDS data is truncated")
	}
	channel := func(v, mask uint32) uint8 {
		if mask == 0 {
			return 0
		}
		shift := uint(0)
		for mask&1 == 0 {
			mask >>= 1
			shift++
		}
		return uint8(((v >> shift) & mask) * 255 / mask)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := body[(y*width+x)*bpp:]
			v := uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16
			if bpp == 4 {
				v |= uint32(p[3]) << 24
			}
			a := uint8(255)
			if pfFlags&ddpfAlphaPixels != 0 {
				a = channel(v, masks[3])
			}
			img.SetNRGBA(x, y, color.NRGBA{R: channel(v, masks[0]), G: channel(v, masks[1]), B: channel(v, masks[2]), A: a})
		}
	}
	return img, nil
}

func rgb565(v uint16) color.NRGBA {
	r := uint8((v >> 11) & 0x1F)
	g := uint8((v >> 5) & 0x3F)
	b := uint8(v & 0x1F)
	return color.NRGBA{R: r<<3 | r>>2, G: g<<2 | g>>4, B: b<<3 | b>>2, A: 255}
}

// decodeColorBlock decodes the 8-byte colour part of a DXT block. DXT1
// blocks with c0 <= c1 use 3 colours plus transparent black.
func decodeColorBlock(b []byte, out *[16]color.NRGBA, dxt1 bool) {
	c0v := binary.LittleEndian.Uint16(b[0:2])
	c1v := binary.LittleEndian.Uint16(b[2:4])
	c0, c1 := rgb565(c0v), rgb565(c1v)

	var palette [4]color.NRGBA
	palette[0], palette[1] = c0, c1
	mix := func(a, b uint8, wa, wb, d int) uint8 { return uint8((int(a)*wa + int(b)*wb) / d) }
	if c0v > c1v || !dxt1 {
		palette[2] = color.NRGBA{R: mix(c0.R, c1.R, 2, 1, 3), G: mix(c0.G, c1.G, 2, 1, 3), B: mix(c0.B, c1.B, 2, 1, 3), A: 255}
		palette[3] = color.NRGBA{R: mix(c0.R, c1.R, 1, 2, 3), G: mix(c0.G, c1.G, 1, 2, 3), B: mix(c0.B, c1.B, 1, 2, 3), A: 255}
	} else {
		palette[2] = color.NRGBA{R: mix(c0.R, c1.R, 1, 1, 2), G: mix(c0.G, c1.G, 1, 1, 2), B: mix(c0.B, c1.B, 1, 1, 2), A: 255}
		palette[3] = color.NRGBA{}
	}

	bits := binary.LittleEndian.Uint32(b[4:8])
	for i := 0; i < 16; i++ {
		out[i] = palette[(bits>>(uint(i)*2))&0x3]
	}
}

// decodeAlphaBlock decodes the 8-byte interpolated alpha of a DXT5 block
func decodeAlphaBlock(b []byte, out *[16]color.NRGBA) {
	a0, a1 := int(b[0]), int(b[1])
	var alphas [8]int
	alphas[0], alphas[1] = a0, a1
	if a0 > a1 {
		for i := 1; i <= 6; i++ {
			alphas[i+1] = ((7-i)*a0 + i*a1) / 7
		}
	} else {
		for i := 1; i <= 4; i++ {
			alphas[i+1] = ((5-i)*a0 + i*a1) / 5
		}
		alphas[6], alphas[7] = 0, 255
	}

	var bits uint64
	for i := 0; i < 6; i++ {
		bits |= uint64(b[2+i]) << (8 * uint(i))
	}
	for i := 0; i < 16; i++ {
		out[i].A = uint8(alphas[(bits>>(uint(i)*3))&0x7])
	}
}
