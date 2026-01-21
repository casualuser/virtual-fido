all: lib dext installer
	mkdir -p output/USBDriverInstaller.app/Contents/DriverExtensions
	mkdir -p output/USBDriverInstaller.app/Contents/Library/SystemExtensions
	cp -r USBDriver/build/build/Release-driverkit/id.bulwark.VirtualUSBDriver.driver.dext output/USBDriverInstaller.app/Contents/DriverExtensions/
	cp -r USBDriver/build/build/Release-driverkit/id.bulwark.VirtualUSBDriver.driver.dext output/USBDriverInstaller.app/Contents/Library/SystemExtensions/
	codesign -s - --force --entitlements USBDriver/USBDriver/USBDriver.entitlements output/USBDriverInstaller.app/Contents/DriverExtensions/id.bulwark.VirtualUSBDriver.driver.dext
	codesign -s - --force --entitlements USBDriver/USBDriver/USBDriver.entitlements output/USBDriverInstaller.app/Contents/Library/SystemExtensions/id.bulwark.VirtualUSBDriver.driver.dext
	codesign -s - --force --entitlements USBDriver/USBDriverInstaller/USBDriverInstaller.entitlements output/USBDriverInstaller.app
	codesign -s - --force output/libUSBDriverLib.a

dext:
	xcodebuild -project USBDriver/USBDriver.xcodeproj -target USBDriver CODE_SIGNING_ALLOWED=NO -configuration Release build

lib: clean
	xcodebuild -project USBDriver/USBDriver.xcodeproj -target USBDriverLib -configuration Release build
	mkdir output
	mv USBDriver/build/build/Release/* ./output/

installer:
	xcodebuild -project USBDriver/USBDriver.xcodeproj -target USBDriverInstaller CODE_SIGNING_ALLOWED=NO -configuration Release build
	cp -r USBDriver/build/build/Release/USBDriverInstaller.app ./output/

clean:
	-rm -rf output
	-rm -rf USBDriver/build