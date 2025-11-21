package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
)

const (
	CardWidthMM  = 54.0
	CardHeightMM = 86.0
	DPI          = 10.0
	CardWidthPx  = int(CardWidthMM * DPI)
	CardHeightPx = int(CardHeightMM * DPI)

	MetaQRSizeMM = 40.0
	MetaQRSizePx = int(MetaQRSizeMM * DPI) // 400px

	SeedQRSizeMM = 40.0
	SeedQRSizePx = int(SeedQRSizeMM * DPI) // 400px
)

var (
	midFace     font.Face
	midFaceOnce sync.Once
	midFaceErr  error
)

func midTextFace() font.Face {
	midFaceOnce.Do(func() {
		fnt, err := opentype.Parse(gomono.TTF)
		if err != nil {
			midFaceErr = err
			return
		}
		midFace, midFaceErr = opentype.NewFace(fnt, &opentype.FaceOptions{
			Size:    19.5, // ~50% larger than basicfont.Face7x13 (~13px)
			DPI:     72,
			Hinting: font.HintingFull,
		})
	})
	if midFaceErr != nil {
		return basicfont.Face7x13
	}
	return midFace
}

// GenerateCardImage creates in-memory PNG image with QR codes and text
// Layout: MetadataQR (top) | Name+FP (middle) | SeedQR (bottom)
func GenerateCardImage(data *CardData) (image.Image, error) {
	// Create image canvas (black background for laser burning)
	img := image.NewGray(image.Rect(0, 0, CardWidthPx, CardHeightPx))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	// Trace line with image hash / date / serial
	drawTextWithFace(img, strings.ToUpper(data.TraceString()), CardWidthPx/2, 20, color.White, basicfont.Face7x13)

	// Generate MetadataQR (xpub:name format)
	metadataStr := data.Xpub
	metaQR, err := qrcode.New(metadataStr, qrcode.Low)
	if err != nil {
		return nil, fmt.Errorf("failed to generate metadata QR: %w", err)
	}
	metaQRImg := metaQR.Image(MetaQRSizePx)

	// Generate SeedQR (mnemonic)
	seedQR, err := qrcode.New(data.Mnemonic, qrcode.Low)
	if err != nil {
		return nil, fmt.Errorf("failed to generate seed QR: %w", err)
	}
	seedQRImg := seedQR.Image(SeedQRSizePx)

	// Layout positions (centered horizontally)
	centerX := CardWidthPx / 2

	// MetadataQR at top (closer to edge)
	metaY := 20 // positioned 2mm from top to make room below
	metaX := centerX - MetaQRSizePx/2
	drawImage(img, metaQRImg, metaX, metaY)

	// Name + fingerprint on one line (uppercase, larger font)
	midLine := fmt.Sprintf("%s %08X", strings.ToUpper(strings.TrimSpace(data.Name)), data.Fingerprint)
	midY := metaY + MetaQRSizePx + 24
	drawTextWithFace(img, midLine, centerX, midY, color.White, midTextFace())

	// SeedQR at bottom edge of card (touching bottom)
	seedY := CardHeightPx - SeedQRSizePx
	seedX := centerX - SeedQRSizePx/2
	drawImage(img, seedQRImg, seedX, seedY)

	// Place seed words below SeedQR in quiet zone (all 12 words in 2 rows, white text on card)
	words := strings.Split(data.Mnemonic, " ")
	if len(words) >= 12 {
		line1 := strings.Join(words[0:6], " ")  // First 6 words
		line2 := strings.Join(words[6:12], " ") // Last 6 words
		// Position in quiet zone below QR code, moved up 23px total
		wordsY := seedY + SeedQRSizePx - 18
		drawTextWithFace(img, line1, centerX, wordsY, color.Black, basicfont.Face7x13)
		drawTextWithFace(img, line2, centerX, wordsY+15, color.Black, basicfont.Face7x13)
	}

	return img, nil
}

// GenerateDescriptorCardImage creates image with descriptor QR for back of card
func GenerateDescriptorCardImage(descriptor string) (image.Image, error) {
	// Create image canvas (black background for laser burning)
	img := image.NewGray(image.Rect(0, 0, CardWidthPx, CardHeightPx))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	// Generate Descriptor QR (larger size for long descriptor string)
	descriptorQR, err := qrcode.New(descriptor, qrcode.Low)
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
	drawTextWithFace(img, "DESCRIPTOR", centerX, labelY, color.White, basicfont.Face7x13)

	return img, nil
}

// drawImage draws a sub-image onto the canvas at position (x, y)
func drawImage(dst *image.Gray, src image.Image, x, y int) {
	r := image.Rect(x, y, x+src.Bounds().Dx(), y+src.Bounds().Dy())
	draw.Draw(dst, r, src, image.Point{}, draw.Over)
}

// drawText draws centered text at the specified position
func drawTextWithFace(img *image.Gray, text string, centerX, y int, col color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
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
func drawTextLeftWithFace(img *image.Gray, text string, x, y int, col color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}
	d.DrawString(text)
}

// drawTextRight draws right-aligned text at the specified position
func drawTextRightWithFace(img *image.Gray, text string, x, y int, col color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
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
