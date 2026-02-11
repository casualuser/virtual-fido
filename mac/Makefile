all: lib

dext:
	xcodebuild -project USBDriver/USBDriver.xcodeproj -target USBDriver ARCHS="arm64e" CODE_SIGN_IDENTITY="" CODE_SIGNING_REQUIRED=NO
	mv USBDriver/build/build/Release-driverkit/* ./output/

lib: clean
	xcodebuild -project USBDriver/USBDriver.xcodeproj -target USBDriverLib -configuration Release
	mkdir output
	mv USBDriver/build/build/Release/* ./output/

clean:
	-rm -r output