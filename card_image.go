package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	CardWidthMM  = 54.0
	CardHeightMM = 86.0
	DPI          = 10.0 // 10 pixels per mm (0.1mm resolution)
	CardWidthPx  = int(CardWidthMM * DPI)  // 540px
	CardHeightPx = int(CardHeightMM * DPI) // 860px

	MetaQRSizeMM = 35.0
	MetaQRSizePx = int(MetaQRSizeMM * DPI) // 350px

	SeedQRSizeMM = 40.0
	SeedQRSizePx = int(SeedQRSizeMM * DPI) // 400px
)

// GenerateCardImage creates in-memory PNG image with QR codes and text
// Layout: MetadataQR (top) | Name+FP (middle) | SeedQR (bottom)
func GenerateCardImage(data *CardData) (image.Image, error) {
	// Create image canvas (black background for laser burning)
	img := image.NewGray(image.Rect(0, 0, CardWidthPx, CardHeightPx))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	// Generate MetadataQR (xpub:name format)
	metadataStr := data.EncodeMetadata()
	metaQR, err := qrcode.New(metadataStr, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to generate metadata QR: %w", err)
	}
	metaQRImg := metaQR.Image(MetaQRSizePx)

	// Generate SeedQR (mnemonic)
	seedQR, err := qrcode.New(data.Mnemonic, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to generate seed QR: %w", err)
	}
	seedQRImg := seedQR.Image(SeedQRSizePx)

	// Layout positions (centered horizontally)
	centerX := CardWidthPx / 2

	// MetadataQR at top (3mm from top)
	metaY := 30 // 3mm from top
	metaX := centerX - MetaQRSizePx/2
	drawImage(img, metaQRImg, metaX, metaY)

	// Name and Fingerprint on same line (aligned to MetadataQR edges)
	textY := metaY + MetaQRSizePx + 20 // 2mm gap

	// Name left-aligned to left edge of MetadataQR
	nameX := metaX
	drawTextLeft(img, data.Name, nameX, textY, color.White)

	// Fingerprint right-aligned to right edge of MetadataQR
	fpX := metaX + MetaQRSizePx
	fpHex := fmt.Sprintf("%08x", data.Fingerprint)
	drawTextRight(img, fpHex, fpX, textY, color.White)

	// SeedQR at bottom edge of card (touching bottom)
	seedY := CardHeightPx - SeedQRSizePx // Position so bottom edge touches card bottom
	seedX := centerX - SeedQRSizePx/2
	drawImage(img, seedQRImg, seedX, seedY)

	// Place seed words below SeedQR in quiet zone (all 12 words in 2 rows, white text on card)
	words := strings.Split(data.Mnemonic, " ")
	if len(words) >= 12 {
		line1 := strings.Join(words[0:6], " ")   // First 6 words
		line2 := strings.Join(words[6:12], " ")  // Last 6 words
		// Position in quiet zone below QR code, moved up 23px total
		wordsY := seedY + SeedQRSizePx - 18  // Move up 23px from original position
		drawText(img, line1, centerX, wordsY, color.Black)      // Black in image = unburned = white text on card
		drawText(img, line2, centerX, wordsY+15, color.Black)   // 1.5mm below first line
	}

	return img, nil
}

// GenerateDescriptorCardImage creates image with descriptor QR for back of card
func GenerateDescriptorCardImage(descriptor string) (image.Image, error) {
	// Create image canvas (black background for laser burning)
	img := image.NewGray(image.Rect(0, 0, CardWidthPx, CardHeightPx))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	// Generate Descriptor QR (larger size for long descriptor string)
	descriptorQR, err := qrcode.New(descriptor, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to generate descriptor QR: %w", err)
	}
	descriptorQRImg := descriptorQR.Image(SeedQRSizePx) // Use same size as SeedQR (400px)

	// Center the QR code on the card
	centerX := CardWidthPx / 2
	centerY := CardHeightPx / 2
	qrX := centerX - SeedQRSizePx/2
	qrY := centerY - SeedQRSizePx/2
	drawImage(img, descriptorQRImg, qrX, qrY)

	// Add "DESCRIPTOR" label above QR
	labelY := qrY - 20 // 2mm above QR
	drawText(img, "DESCRIPTOR", centerX, labelY, color.White)

	return img, nil
}

// drawImage draws a sub-image onto the canvas at position (x, y)
func drawImage(dst *image.Gray, src image.Image, x, y int) {
	r := image.Rect(x, y, x+src.Bounds().Dx(), y+src.Bounds().Dy())
	draw.Draw(dst, r, src, image.Point{}, draw.Over)
}

// drawText draws centered text at the specified position
func drawText(img *image.Gray, text string, centerX, y int, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(centerX * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}

	// Center the text
	textWidth := d.MeasureString(text)
	d.Dot.X -= textWidth / 2
	d.DrawString(text)
}

// drawTextLeft draws left-aligned text at the specified position
func drawTextLeft(img *image.Gray, text string, x, y int, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}
	d.DrawString(text)
}

// drawTextRight draws right-aligned text at the specified position
func drawTextRight(img *image.Gray, text string, x, y int, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}

	// Measure text and subtract from X position for right alignment
	textWidth := d.MeasureString(text)
	d.Dot.X -= textWidth
	d.DrawString(text)
}
