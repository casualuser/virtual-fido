#include "HIDVirtualDevice/HIDVirtualDeviceBridge.h"
#include "_cgo_export.h"
#include <stdint.h>
#include <stdlib.h>

// Wrapper to match the bridge callback signature (const uint8_t*) and call Go
void bridgeCallbackWrapper(const uint8_t *data, size_t length) {
  HIDReceiveReport((unsigned char *)data, length);
}
