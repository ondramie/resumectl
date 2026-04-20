TEAM_ID ?= 4WM77L2S6H
DEVICE_ID ?= C3527516-7F1D-52B6-8FF6-CC68584B9774

.PHONY: generate build install clean

generate:
	xcodegen generate

build: generate
	xcodebuild -project Resumectl.xcodeproj \
		-scheme Resumectl \
		-destination 'id=$(DEVICE_ID)' \
		-configuration Debug \
		DEVELOPMENT_TEAM=$(TEAM_ID) \
		CODE_SIGN_STYLE=Automatic \
		-allowProvisioningUpdates \
		build

install: generate
	xcodebuild -project Resumectl.xcodeproj \
		-scheme Resumectl \
		-destination 'id=$(DEVICE_ID)' \
		-configuration Debug \
		DEVELOPMENT_TEAM=$(TEAM_ID) \
		CODE_SIGN_STYLE=Automatic \
		-allowProvisioningUpdates \
		build && \
	APP=$$(find ~/Library/Developer/Xcode/DerivedData -name "Resumectl.app" -not -path "*/Index.noindex/*" -path "*/Debug-iphoneos/*" 2>/dev/null | head -1); \
	xcrun devicectl device install app --device $(DEVICE_ID) "$$APP"

clean:
	rm -rf Resumectl.xcodeproj build
