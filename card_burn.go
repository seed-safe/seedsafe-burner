package main

import (
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"runtime"
	"strings"
	"time"

	"go.bug.st/serial"
)

// BurnSettings contains laser configuration
type BurnSettings struct {
	Port       string
	Baud       int
	TracePower int
	TraceSpeed int
	BurnPower  int
	BurnSpeed  int
	MockMode   bool
}

// DetectSerialPorts scans for available USB serial devices
func DetectSerialPorts() []string {
	var ports []string

	if runtime.GOOS == "darwin" {
		// macOS: Check /dev/tty.* for USB serial devices
		files, err := ioutil.ReadDir("/dev")
		if err != nil {
			return ports
		}

		for _, f := range files {
			name := f.Name()
			// Common USB serial device patterns
			if strings.HasPrefix(name, "tty.usbserial") ||
				strings.HasPrefix(name, "tty.usbmodem") ||
				strings.HasPrefix(name, "tty.wchusbserial") ||
				strings.HasPrefix(name, "cu.usbserial") ||
				strings.HasPrefix(name, "cu.usbmodem") ||
				strings.HasPrefix(name, "cu.wchusbserial") {
				ports = append(ports, "/dev/"+name)
			}
		}
	} else if runtime.GOOS == "linux" {
		// Linux: Check /dev/ttyUSB*, /dev/ttyACM*
		files, err := ioutil.ReadDir("/dev")
		if err != nil {
			return ports
		}

		for _, f := range files {
			name := f.Name()
			if strings.HasPrefix(name, "ttyUSB") ||
				strings.HasPrefix(name, "ttyACM") {
				ports = append(ports, "/dev/"+name)
			}
		}
	}

	return ports
}

// TestSerialPort attempts to open and verify a serial port
func TestSerialPort(port string, baud int) bool {
	mode := &serial.Mode{
		BaudRate: baud,
	}

	s, err := serial.Open(port, mode)
	if err != nil {
		return false
	}
	defer s.Close()

	// Try to get GRBL status
	s.Write([]byte("?\n"))
	time.Sleep(100 * time.Millisecond)
	buf := make([]byte, 128)
	n, err := s.Read(buf)
	if err != nil {
		return false
	}

	// Check if response looks like GRBL
	response := string(buf[:n])
	return strings.Contains(response, "<") || strings.Contains(response, "ok")
}

// AutoDetectLaser scans for and tests laser engraver connection
func AutoDetectLaser(baud int) (string, bool) {
	fmt.Println("\n─── Auto-detecting laser engraver ───")

	ports := DetectSerialPorts()
	if len(ports) == 0 {
		fmt.Println("  No USB serial devices found")
		return "", false
	}

	fmt.Printf("  Found %d serial port(s):\n", len(ports))
	for _, port := range ports {
		fmt.Printf("    - %s\n", port)
	}

	// Test each port
	for _, port := range ports {
		fmt.Printf("  Testing %s... ", port)
		if TestSerialPort(port, baud) {
			fmt.Println("✓ GRBL detected!")
			return port, true
		}
		fmt.Println("✗")
	}

	fmt.Println("  No GRBL devices detected")
	return "", false
}

// GetDefaultSerialPort returns platform-specific default
func GetDefaultSerialPort() string {
	if runtime.GOOS == "darwin" {
		return "/dev/tty.usbserial"
	}
	return "/dev/ttyUSB0"
}

// ImageToGCode converts in-memory image to G-code (serpentine raster)
func ImageToGCode(img image.Image, power int, speed int) []byte {
	bounds := img.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	const scale = 0.1  // 0.1mm per pixel
	const xOffset = 0.0

	var gcode bytes.Buffer

	// Header
	gcode.WriteString("G21\n")       // mm mode
	gcode.WriteString("G90\n")       // absolute positioning
	gcode.WriteString("M5\n")        // laser off
	gcode.WriteString("G92 X0 Y0\n") // zero work coordinates
	gcode.WriteString(fmt.Sprintf("G0 X%.3f Y0.000\n", xOffset))
	gcode.WriteString("M4 S0\n") // dynamic laser mode
	gcode.WriteString(fmt.Sprintf("F%d\n", speed))

	// Print progress preview as rows are processed
	fmt.Println("\n─── Generating G-code with preview ───")
	totalRows := height

	// Process rows from bottom to top for G-code generation
	for row := height - 1; row >= 0; row-- {
		rowNum := height - 1 - row

		// Show progress every 10 rows with detailed preview
		if rowNum%10 == 0 {
			// Build visual representation by sampling blocks of pixels
			// Each character represents multiple pixels with density shading
			var rowViz strings.Builder
			displayWidth := 100
			blockWidth := width / displayWidth // Pixels per character (~5-6 pixels)

			for i := 0; i < displayWidth; i++ {
				// Sample a block of pixels
				startX := i * blockWidth
				endX := startX + blockWidth
				if endX > width {
					endX = width
				}

				// Count white pixels (will burn) in this block
				whiteCount := 0
				totalPixels := endX - startX
				for x := startX; x < endX; x++ {
					if isBlackPixel(img, x, row) {
						whiteCount++
					}
				}

				// Calculate density and choose character
				density := float64(whiteCount) / float64(totalPixels)
				var char string
				if density == 0 {
					char = " " // No burning (stays black)
				} else if density < 0.25 {
					char = "░" // Light burn (25%)
				} else if density < 0.50 {
					char = "▒" // Medium burn (50%)
				} else if density < 0.75 {
					char = "▓" // Heavy burn (75%)
				} else {
					char = "█" // Full burn (100%)
				}
				rowViz.WriteString(char)
			}
			fmt.Printf("  Row %4d/%d [%3.0f%%] %s\n", rowNum, totalRows, float64(rowNum)/float64(totalRows)*100, rowViz.String())
		}

		// Check if row has any black pixels
		hasBlack := false
		for x := 0; x < width; x++ {
			if isBlackPixel(img, x, row) {
				hasBlack = true
				break
			}
		}
		if !hasBlack {
			continue // Skip all-white rows
		}

		y := float64(height-1-row) * scale

		// Move to start of this scan line (rapid move with laser off)
		gcode.WriteString(fmt.Sprintf("G0 Y%.3f\n", y))

		// Serpentine: alternate scan direction
		leftToRight := (height-1-row)%2 == 0

		var cols []int
		if leftToRight {
			cols = make([]int, width)
			for i := 0; i < width; i++ {
				cols[i] = i
			}
		} else {
			cols = make([]int, width)
			for i := 0; i < width; i++ {
				cols[i] = width - 1 - i
			}
		}

		// Run-length encoding: group consecutive black pixels
		inBlackRun := false
		runStartCol := -1

		for _, col := range cols {
			black := isBlackPixel(img, col, row)

			if black && !inBlackRun {
				runStartCol = col
				inBlackRun = true
			} else if !black && inBlackRun {
				// End of black run - generate burn segment
				var xStart, xEnd float64
				if leftToRight {
					xStart = float64(runStartCol)*scale + xOffset
					xEnd = float64(col)*scale + xOffset
				} else {
					xStart = float64(runStartCol+1)*scale + xOffset
					xEnd = float64(col+1)*scale + xOffset
				}

				gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S0\n", xStart, y))
				gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", xEnd, y, power))

				inBlackRun = false
			}
		}

		// Handle black run extending to end of row
		if inBlackRun {
			var xStart, xEnd float64
			if leftToRight {
				xStart = float64(runStartCol)*scale + xOffset
				xEnd = float64(width)*scale + xOffset
			} else {
				xStart = float64(runStartCol+1)*scale + xOffset
				xEnd = xOffset
			}

			gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S0\n", xStart, y))
			gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", xEnd, y, power))
		}
	}

	fmt.Println("  ✓ G-code generation complete\n")

	// Footer
	gcode.WriteString("M5\n")
	gcode.WriteString(fmt.Sprintf("G0 X%.3f Y0.000\n", xOffset))
	gcode.WriteString("M2\n")

	return gcode.Bytes()
}

func isBlackPixel(img image.Image, x, y int) bool {
	gray, _, _, _ := img.At(x, y).RGBA()
	// Inverted: burn WHITE pixels (exposing silver), leave BLACK pixels (staying black)
	// For black anodized aluminum cards: burning removes anodizing (black->silver)
	return gray >= 32768 // Threshold: >= 50% brightness = white = burn it
}

// BurnToLaser sends G-code to USB serial (with mock mode fallback)
func BurnToLaser(gcode []byte, settings *BurnSettings) error {
	if settings.MockMode {
		fmt.Println("\n[MOCK MODE] Simulating laser burn...")
		lines := bytes.Split(gcode, []byte("\n"))
		fmt.Printf("  Would send %d G-code lines\n", len(lines))
		fmt.Println("  ✓ Mock burn complete!")
		return nil
	}

	mode := &serial.Mode{
		BaudRate: settings.Baud,
	}

	s, err := serial.Open(settings.Port, mode)
	if err != nil {
		fmt.Printf("✗ Failed to open %s: %v\n", settings.Port, err)
		fmt.Println("  Tip: Check USB connection or enable mock mode")
		return err
	}
	defer s.Close()

	// Set read timeout (shorter for better responsiveness)
	s.SetReadTimeout(500 * time.Millisecond)

	// Send wake-up command to GRBL
	s.Write([]byte("\r\n\r\n"))
	time.Sleep(2 * time.Second)

	// Clear any startup messages
	buf := make([]byte, 256)
	s.Read(buf)

	fmt.Println("  GRBL initialized and ready")

	lines := bytes.Split(gcode, []byte("\n"))
	total := len(lines)

	startTime := time.Now()
	var totalWaitTime time.Duration
	var timeoutCount int
	var errorCount int

	fmt.Println("\n─── Burn Performance Log ───")

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Send command
		_, err := s.Write(append(line, '\n'))
		if err != nil {
			return fmt.Errorf("send error line %d: %w", i, err)
		}

		// Wait for "ok" from GRBL (with proper flow control)
		responseStart := time.Now()
		retries := 0

		for {
			buf := make([]byte, 256)
			n, err := s.Read(buf)
			if err != nil {
				// Timeout or error - retry
				retries++
				timeoutCount++
				if retries > 5 {
					fmt.Printf("\n  ERROR: Line %d - No response after 5 retries\n", i)
					fmt.Printf("  Command was: %s\n", string(line))
					errorCount++
					break
				}
				continue
			}

			response := strings.TrimSpace(string(buf[:n]))

			// Log any non-standard responses
			if !strings.Contains(response, "ok") {
				fmt.Printf("\n  Line %d response: %s\n", i, response)
			}

			// Check if we got "ok" response
			if strings.Contains(response, "ok") {
				break // Got confirmation, send next command
			}

			// If we got an error response, log it
			if strings.Contains(response, "error") {
				fmt.Printf("\n  ERROR: Line %d - GRBL error: %s\n", i, response)
				fmt.Printf("  Command was: %s\n", string(line))
				errorCount++
				break
			}

			// If we got alarm response
			if strings.Contains(response, "ALARM") {
				fmt.Printf("\n  ALARM: Line %d - %s\n", i, response)
				return fmt.Errorf("GRBL alarm at line %d: %s", i, response)
			}
		}

		waitTime := time.Since(responseStart)
		totalWaitTime += waitTime

		// Detailed progress with timing every 50 lines
		if i%50 == 0 {
			avgWaitMs := float64(totalWaitTime.Milliseconds()) / float64(i+1)
			fmt.Printf("\r  Progress: %d/%d (%.1f%%) | Avg wait: %.1fms | Timeouts: %d | Errors: %d",
				i, total, float64(i)/float64(total)*100, avgWaitMs, timeoutCount, errorCount)
		}

		// Log slow responses
		if waitTime > 500*time.Millisecond {
			fmt.Printf("\n  SLOW: Line %d took %dms to respond\n", i, waitTime.Milliseconds())
		}
	}

	totalTime := time.Since(startTime)
	fmt.Printf("\n\n─── Burn Statistics ───\n")
	fmt.Printf("  Total time: %s\n", totalTime)
	fmt.Printf("  Lines sent: %d\n", total)
	fmt.Printf("  Avg response time: %.1fms\n", float64(totalWaitTime.Milliseconds())/float64(total))
	fmt.Printf("  Timeouts: %d\n", timeoutCount)
	fmt.Printf("  Errors: %d\n", errorCount)
	fmt.Printf("  Lines/sec: %.1f\n", float64(total)/totalTime.Seconds())

	fmt.Println("\n✓ Burn complete!")
	return nil
}

// TraceFrame outlines card boundaries
func TraceFrame(settings *BurnSettings) error {
	var gcode bytes.Buffer

	const xOffset = 0.0

	gcode.WriteString("G21\nG90\n")
	gcode.WriteString(fmt.Sprintf("G0 X%.1f Y0\n", xOffset))
	gcode.WriteString(fmt.Sprintf("F%d\nM3\nS%d\n", settings.TraceSpeed, settings.TracePower))
	gcode.WriteString(fmt.Sprintf("G1 X%.1f Y0\n", 54+xOffset))
	gcode.WriteString(fmt.Sprintf("G1 X%.1f Y86\n", 54+xOffset))
	gcode.WriteString(fmt.Sprintf("G1 X%.1f Y86\n", xOffset))
	gcode.WriteString(fmt.Sprintf("G1 X%.1f Y0\n", xOffset))
	gcode.WriteString("S0\nM5\n")

	return BurnToLaser(gcode.Bytes(), settings)
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateHatchPattern creates a hatch fill pattern for burning a pocket in acrylic
// This creates horizontal scan lines to remove material and create a recess
func GenerateHatchPattern(power int, speed int) []byte {
	const xOffset float64 = 0.0
	const pocketWidth float64 = 56.0  // mm (extra clearance)
	const pocketHeight float64 = 88.0 // mm (extra clearance)
	const lineSpacing float64 = 0.2   // mm between hatch lines (more overlap)

	var gcode bytes.Buffer

	// Header
	gcode.WriteString("G21\n")                     // mm mode
	gcode.WriteString("G90\n")                     // absolute positioning
	gcode.WriteString("M5\n")                      // laser off
	gcode.WriteString("G92 X0 Y0\n")               // zero work coordinates
	gcode.WriteString("M4 S0\n")                   // dynamic laser mode
	gcode.WriteString(fmt.Sprintf("F%d\n", speed)) // feed rate

	fmt.Println("  Generating hatch pattern...")

	// Calculate number of passes
	numPassesFloat := pocketHeight / lineSpacing
	numPasses := int(numPassesFloat)
	fmt.Printf("  Lines: %d (%.1fmm spacing)\n", numPasses, lineSpacing)

	// Generate horizontal scan lines (serpentine pattern)
	for i := 0; i < numPasses; i++ {
		y := float64(i) * lineSpacing
		leftToRight := i%2 == 0

		// Move to start of line (laser off)
		if leftToRight {
			gcode.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f S0\n", xOffset, y))
			// Burn across (left to right)
			gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", xOffset+pocketWidth, y, power))
		} else {
			gcode.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f S0\n", xOffset+pocketWidth, y))
			// Burn across (right to left)
			gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", xOffset, y, power))
		}

		// Progress indicator
		if i%50 == 0 {
			fmt.Printf("  Progress: %d/%d [%3.0f%%]\n", i, numPasses, float64(i)/float64(numPasses)*100)
		}
	}

	fmt.Printf("  Progress: %d/%d [100%%]\n", numPasses, numPasses)

	// Footer
	gcode.WriteString("M5\n")                                    // laser off
	gcode.WriteString(fmt.Sprintf("G0 X%.3f Y0.000\n", xOffset)) // return to start
	gcode.WriteString("M2\n")                                    // program end

	return gcode.Bytes()
}

// GeneratePocketOutlineGCode generates a single-pass perimeter cut for the acrylic pocket.
func GeneratePocketOutlineGCode(power int, speed int) []byte {
	const xOffset = 0.0
	const pocketWidth = 55.0
	const pocketHeight = 87.0
	const cornerRadius = 3.0

	var gcode bytes.Buffer
	gcode.WriteString("G21\n")
	gcode.WriteString("G90\n")
	gcode.WriteString("M5\n")
	gcode.WriteString("G92 X0 Y0\n")
	gcode.WriteString("M4 S0\n")
	gcode.WriteString(fmt.Sprintf("F%d\n", speed))

	left := xOffset
	right := xOffset + pocketWidth
	bottom := 0.0
	top := pocketHeight

	startX := left + cornerRadius
	startY := bottom

	gcode.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f S0\n", startX, startY))

	// Bottom edge
	gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", right-cornerRadius, bottom, power))
	// Bottom-right corner (CCW)
	gcode.WriteString(fmt.Sprintf("G3 X%.3f Y%.3f I0 J%.3f S%d\n", right, bottom+cornerRadius, cornerRadius, power))
	// Right edge
	gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", right, top-cornerRadius, power))
	// Top-right corner
	gcode.WriteString(fmt.Sprintf("G3 X%.3f Y%.3f I-%.3f J0 S%d\n", right-cornerRadius, top, cornerRadius, power))
	// Top edge
	gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", left+cornerRadius, top, power))
	// Top-left corner
	gcode.WriteString(fmt.Sprintf("G3 X%.3f Y%.3f I0 J-%.3f S%d\n", left, top-cornerRadius, cornerRadius, power))
	// Left edge
	gcode.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f S%d\n", left, bottom+cornerRadius, power))
	// Bottom-left corner
	gcode.WriteString(fmt.Sprintf("G3 X%.3f Y%.3f I%.3f J0 S%d\n", left+cornerRadius, bottom, cornerRadius, power))

	gcode.WriteString("M5\n")
	gcode.WriteString(fmt.Sprintf("G0 X%.3f Y0.000\n", xOffset))
	gcode.WriteString("M2\n")
	return gcode.Bytes()
}
