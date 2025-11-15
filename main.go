package main

import (
	"bufio"
	"flag"
	"fmt"
	"image/png"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var pngOutputFile string

func main() {
	// Parse command-line flags
	flag.StringVar(&pngOutputFile, "f", "", "Save generated PNG to file")
	flag.Parse()

	fmt.Printf("\n═══════════════════════════════\n")
	fmt.Printf("  SeedSafe Card Burner\n")
	fmt.Printf("  Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if pngOutputFile != "" {
		fmt.Printf("  PNG Output: %s\n", pngOutputFile)
	}
	fmt.Printf("═══════════════════════════════\n")

	// Auto-detect laser engraver
	detectedPort, found := AutoDetectLaser(115200)

	var settings *BurnSettings
	if found {
		fmt.Printf("\n✓ Using laser at %s\n", detectedPort)
		settings = &BurnSettings{
			Port:       detectedPort,
			Baud:       115200,
			TracePower: 50,
			TraceSpeed: 2000,
			BurnPower:  300,
			BurnSpeed:  1000,
			MockMode:   false,
		}
	} else {
		fmt.Println("\n⚠ No laser detected - using MOCK mode")
		fmt.Println("  (G-code will be generated but not sent)")
		settings = &BurnSettings{
			Port:       GetDefaultSerialPort(),
			Baud:       115200,
			TracePower: 50,
			TraceSpeed: 2000,
			BurnPower:  300,
			BurnSpeed:  1000,
			MockMode:   true,
		}
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("\n┌──────────────────────────────────┐")
		fmt.Println("│  Main Menu                       │")
		fmt.Println("├──────────────────────────────────┤")
		fmt.Println("│ 1. Burn card (front side key)    │")
		fmt.Println("│ 2. Burn card (back descriptor)   │")
		fmt.Println("│ 3. Burn card pocket              │")
		fmt.Println("│ 4. Trace frame only              │")
		fmt.Println("│ 5. Re-detect laser               │")
		fmt.Println("│ 6. Settings                      │")
		fmt.Println("│ 7. Exit                          │")
		fmt.Println("└──────────────────────────────────┘")

		if settings.MockMode {
			fmt.Println("  [MOCK MODE]")
		} else {
			fmt.Printf("  [%s]\n", settings.Port)
		}

		fmt.Print("\nSelect (1-7): ")

		scanner.Scan()
		choice := scanner.Text()

		switch choice {
		case "1":
			burnCard(settings)
		case "2":
			generateDescriptor(settings)
		case "3":
			burnCardPocket(settings)
		case "4":
			traceFrame(settings)
		case "5":
			redetectLaser(settings)
		case "6":
			configureSettings(settings, scanner)
		case "7":
			fmt.Println("Goodbye!")
			return
		}
	}
}

func redetectLaser(settings *BurnSettings) {
	detectedPort, found := AutoDetectLaser(settings.Baud)
	if found {
		settings.Port = detectedPort
		settings.MockMode = false
		fmt.Printf("✓ Laser configured at %s\n", detectedPort)
	} else {
		fmt.Println("⚠ No laser found")
		fmt.Print("Enable mock mode? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			settings.MockMode = true
			fmt.Println("✓ Mock mode enabled")
		}
	}
}

func burnCard(settings *BurnSettings) {
	fmt.Println("\n─── Generating Card ───")
	data, err := GenerateCardData()
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		return
	}

	fmt.Printf("  Name: %s\n", data.Name)
	fmt.Printf("  Fingerprint: %08x\n", data.Fingerprint)

	fmt.Println("\n─── Rendering Image ───")
	img, err := GenerateCardImage(data)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		return
	}
	fmt.Printf("  ✓ %dx%d pixels\n", img.Bounds().Dx(), img.Bounds().Dy())

	// Save PNG if -f flag was provided
	if pngOutputFile != "" {
		fmt.Printf("\n─── Saving PNG ───\n")
		f, err := os.Create(pngOutputFile)
		if err != nil {
			fmt.Printf("✗ Error creating file: %v\n", err)
			return
		}
		defer f.Close()

		err = png.Encode(f, img)
		if err != nil {
			fmt.Printf("✗ Error encoding PNG: %v\n", err)
			return
		}
		fmt.Printf("  ✓ Saved to %s\n", pngOutputFile)
	}

	gcode := ImageToGCode(img, settings.BurnPower, settings.BurnSpeed)
	fmt.Printf("  ✓ Generated %d bytes of G-code\n", len(gcode))

	if settings.MockMode {
		fmt.Print("\nPress Enter to simulate burn... ")
	} else {
		fmt.Print("\nPress Enter to START BURNING... ")
	}
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	fmt.Println("\n─── Burning ───")
	err = BurnToLaser(gcode, settings)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
	}
}

func traceFrame(settings *BurnSettings) {
	fmt.Println("\n─── Tracing Frame ───")
	err := TraceFrame(settings)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
	}
}

func generateDescriptor(settings *BurnSettings) {
	fmt.Println("\n─── 3-2-1 Taproot Descriptor Generation ───")
	fmt.Println("This will create a Taproot timelock descriptor with:")
	fmt.Println("  • 3-of-N multisig (immediate)")
	fmt.Println("  • 2-of-N multisig (after timelock 1)")
	fmt.Println("  • 1-of-N multisig (after timelock 2)")
	fmt.Println("")

	// Step 1: Choose duration
	fmt.Println("Select timelock duration:")
	fmt.Println("  1. Production (1 year / 1.25 years)")
	fmt.Println("  2. Testing (1 hour / 1.5 hours)")
	fmt.Print("\nChoice (1-2): ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	durationChoice := scanner.Text()

	duration := "year"
	if durationChoice == "2" {
		duration = "hour"
		fmt.Println("✓ Using testing timelock (1 hour / 1.5 hours)")
	} else {
		fmt.Println("✓ Using production timelock (1 year / 1.25 years)")
	}

	// Step 2: Collect xpubs from user
	xpubs, err := CollectXpubsManual()
	if err != nil {
		fmt.Printf("✗ Error collecting xpubs: %v\n", err)
		return
	}

	fmt.Println("\n─── Collected Xpubs ───")
	for i, xpub := range xpubs {
		// Show first 20 and last 10 chars of each xpub
		if len(xpub) > 30 {
			fmt.Printf("%d. %s...%s\n", i+1, xpub[:20], xpub[len(xpub)-10:])
		} else {
			fmt.Printf("%d. %s\n", i+1, xpub)
		}
	}

	// Step 3: Generate 3-2-1 Taproot descriptor
	fmt.Println("\n─── Generating 3-2-1 Descriptor ───")
	descriptor, err := Generate321Descriptor(xpubs, duration)
	if err != nil {
		fmt.Printf("✗ Error generating descriptor: %v\n", err)
		return
	}

	fmt.Printf("  Signers: %d\n", len(xpubs))
	fmt.Printf("  Type: Taproot (3-2-1 timelock)\n")
	fmt.Printf("  Descriptor length: %d chars\n", len(descriptor))

	// Show descriptor (truncated if too long)
	if len(descriptor) > 200 {
		fmt.Printf("  Descriptor: %s...%s\n", descriptor[:100], descriptor[len(descriptor)-50:])
	} else {
		fmt.Printf("  Descriptor: %s\n", descriptor)
	}

	// Step 4: Generate descriptor QR image
	fmt.Println("\n─── Rendering Descriptor Image ───")
	img, err := GenerateDescriptorCardImage(descriptor)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		return
	}
	fmt.Printf("  ✓ %dx%d pixels\n", img.Bounds().Dx(), img.Bounds().Dy())

	// Save PNG if -f flag was provided
	if pngOutputFile != "" {
		fmt.Printf("\n─── Saving Descriptor PNG ───\n")
		// Append "_descriptor" to filename
		descriptorFile := strings.Replace(pngOutputFile, ".png", "_descriptor.png", 1)
		f, err := os.Create(descriptorFile)
		if err != nil {
			fmt.Printf("✗ Error creating file: %v\n", err)
			return
		}
		defer f.Close()

		err = png.Encode(f, img)
		if err != nil {
			fmt.Printf("✗ Error encoding PNG: %v\n", err)
			return
		}
		fmt.Printf("  ✓ Saved to %s\n", descriptorFile)
	}

	// Step 5: Convert to G-code
	gcode := ImageToGCode(img, settings.BurnPower, settings.BurnSpeed)
	fmt.Printf("  ✓ Generated %d bytes of G-code\n", len(gcode))

	// Step 6: Burn to card
	fmt.Println("\nREADY TO BURN DESCRIPTOR TO BACK OF CARD")
	if settings.MockMode {
		fmt.Print("Press Enter to simulate burn... ")
	} else {
		fmt.Print("Press Enter to START BURNING... ")
	}
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	fmt.Println("\n─── Burning ───")
	err = BurnToLaser(gcode, settings)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
	}
}

func burnCardPocket(settings *BurnSettings) {
	fmt.Println("\n─── Card Pocket Hatch Burn ───")
	fmt.Println("This will burn a hatch pattern into black acrylic")
	fmt.Println("to create a pocket for holding the card.")
	fmt.Println("")
	fmt.Printf("Pocket size: 55mm × 87mm (card 54×86mm + 1mm clearance)\n")
	fmt.Printf("Hatch spacing: 0.3mm\n")
	fmt.Printf("Power: 800 (80%% for 5W diode)\n")
	fmt.Printf("Speed: 200 mm/min (slow for deeper burn)\n")
	fmt.Println("")

	// Generate hatch pattern G-code
	fmt.Println("─── Generating Hatch G-code ───")
	gcode := GenerateHatchPattern(800, 200)
	fmt.Printf("  ✓ Generated %d bytes of G-code\n", len(gcode))

	if settings.MockMode {
		fmt.Print("\nPress Enter to simulate burn... ")
	} else {
		fmt.Print("\nPress Enter to START BURNING POCKET... ")
	}
	bufio.NewReader(os.Stdin).ReadBytes('\n')

	fmt.Println("\n─── Burning Pocket ───")
	err := BurnToLaser(gcode, settings)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
	}
}

func configureSettings(settings *BurnSettings, scanner *bufio.Scanner) {
	fmt.Println("\n─── Settings ───")
	fmt.Printf("Current configuration:\n")
	fmt.Printf("  Port: %s\n", settings.Port)
	fmt.Printf("  Baud: %d\n", settings.Baud)
	fmt.Printf("  Trace power: %d\n", settings.TracePower)
	fmt.Printf("  Trace speed: %d mm/min\n", settings.TraceSpeed)
	fmt.Printf("  Burn power: %d\n", settings.BurnPower)
	fmt.Printf("  Burn speed: %d mm/min\n", settings.BurnSpeed)
	fmt.Printf("  Mock mode: %v\n", settings.MockMode)

	fmt.Println("\n1. Change port")
	fmt.Println("2. Change burn power")
	fmt.Println("3. Change burn speed")
	fmt.Println("4. Toggle mock mode")
	fmt.Println("5. List available ports")
	fmt.Println("6. Back")

	fmt.Print("\nSelect: ")
	scanner.Scan()
	choice := scanner.Text()

	switch choice {
	case "1":
		fmt.Print("Enter port path: ")
		scanner.Scan()
		settings.Port = scanner.Text()
	case "2":
		fmt.Print("Burn power (0-1000): ")
		scanner.Scan()
		if val, err := strconv.Atoi(scanner.Text()); err == nil {
			settings.BurnPower = val
		}
	case "3":
		fmt.Print("Burn speed (mm/min): ")
		scanner.Scan()
		if val, err := strconv.Atoi(scanner.Text()); err == nil {
			settings.BurnSpeed = val
		}
	case "4":
		settings.MockMode = !settings.MockMode
		fmt.Printf("Mock mode: %v\n", settings.MockMode)
	case "5":
		ports := DetectSerialPorts()
		fmt.Printf("\nAvailable ports:\n")
		for _, port := range ports {
			fmt.Printf("  - %s\n", port)
		}
	}
}
