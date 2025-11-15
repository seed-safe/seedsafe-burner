# Makefile for SeedSafe Card Burner

.PHONY: all build-mac build-rpi build-rpi-static clean run test deps deploy

# Default: build for current platform
all: build

# Build for current platform
build:
	go build -o seedsafe .

# Build for macOS (dev/testing)
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o seedsafe-darwin .

# Build for Raspberry Pi Zero 2W (ARMv7)
build-rpi:
	GOOS=linux GOARCH=arm GOARM=7 go build -o seedsafe-linux-arm .

# Build for RPi with static linking (portable)
build-rpi-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -a -ldflags '-extldflags "-static"' -o seedsafe-linux-arm .

# Run on current platform
run:
	go run .

# Test build on macOS
test: build-mac
	./seedsafe-darwin

# Clean build artifacts
clean:
	rm -f seedsafe seedsafe-darwin seedsafe-linux-arm

# Install dependencies
deps:
	go mod download
	go mod tidy

# Deploy to RPi (adjust IP/hostname as needed)
deploy: build-rpi
	scp seedsafe-linux-arm pi@raspberrypi.local:~/seedsafe
	ssh pi@raspberrypi.local 'chmod +x ~/seedsafe'

# Show help
help:
	@echo "SeedSafe Card Burner - Build Targets"
	@echo ""
	@echo "  make build          - Build for current platform"
	@echo "  make build-mac      - Build for macOS (ARM64)"
	@echo "  make build-rpi      - Build for Raspberry Pi Zero 2W"
	@echo "  make run            - Run locally"
	@echo "  make test           - Build and test on macOS"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make deps           - Download dependencies"
	@echo "  make deploy         - Build and deploy to RPi"
